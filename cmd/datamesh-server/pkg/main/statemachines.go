package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/nu7hatch/gouuid"
	"golang.org/x/net/context"
)

func newFilesystemMachine(filesystemId string, s *InMemoryState) *fsMachine {
	// initialize the fsMachine with a filesystem struct that has bare minimum
	// information (just the filesystem id) required to get started
	return &fsMachine{
		filesystem: &filesystem{
			id: filesystemId,
		},
		// stored here as well to avoid excessive locking on filesystem struct,
		// which gets clobbered, just to read its id
		filesystemId:            filesystemId,
		requests:                make(chan *Event),
		innerRequests:           make(chan *Event),
		innerResponses:          make(chan *Event),
		responses:               map[string]chan *Event{},
		responsesLock:           &sync.Mutex{},
		snapshotsModified:       make(chan bool),
		state:                   s,
		snapshotsLock:           &sync.Mutex{},
		newSnapsOnServers:       NewObserver(),
		currentState:            "discovering",
		status:                  "",
		lastTransitionTimestamp: time.Now().UnixNano(),
		transitionObserver:      NewObserver(),
		lastTransferRequest:     TransferRequest{},
		// In the case where we're receiving a push (pushPeerState), it's the
		// POST handler on our http server which handles the receiving of the
		// snapshot. We need to coordinate with it so that we know when to
		// reload the list of snapshots, update etcd and coordinate our own
		// state changes, which we do via the POST handler sending on this
		// channel.
		externalSnapshotsChanged: make(chan bool),
		dirtyDelta:               0,
		sizeBytes:                0,
	}
}

func (f *fsMachine) run() {
	// TODO cancel this when we eventually support deletion
	log.Printf("[run:%s] INIT", f.filesystemId)
	go runForever(
		f.updateEtcdAboutSnapshots,
		"updateEtcdAboutSnapshots",
		1*time.Second,
		1*time.Second,
	)
	go runForever(
		f.pollDirty,
		"pollDirty",
		1*time.Second,
		1*time.Second,
	)
	go func() {
		for state := discoveringState; state != nil; {
			state = state(f)
		}
		log.Printf("[run:%s] got nil state, closing up shop", f.filesystemId)
		close(f.requests) // no more events will come out if we reach the nil state
		close(f.innerRequests)
		close(f.innerResponses)
		f.responsesLock.Lock()
		for _, c := range f.responses {
			close(c)
		}
		f.responsesLock.Unlock()
		close(f.snapshotsModified)
	}()
	// proxy requests and responses, enforcing an ordering, to avoid accepting
	// a new request before a response comes back, ie to serialize requests &
	// responses per-statemachine (without blocking the entire etcd event loop,
	// which asynchronously writes to the requests chan)
	log.Printf("[run:%s] reading from external requests", f.filesystemId)
	for req := range f.requests {
		log.Printf("[run:%s] got req: %s", f.filesystemId, req)
		log.Printf("[run:%s] writing to internal requests", f.filesystemId)
		f.innerRequests <- req
		log.Printf("[run:%s] reading from internal responses", f.filesystemId)
		resp := <-f.innerResponses
		log.Printf("[run:%s] got resp: %s", f.filesystemId, resp)
		log.Printf("[run:%s] writing to external responses", f.filesystemId)
		f.responsesLock.Lock()
		respChan, ok := f.responses[(*req.Args)["RequestId"].(string)]
		f.responsesLock.Unlock()
		if ok {
			respChan <- resp
		} else {
			log.Printf(
				"[run:%s] unable to find response chan '%s'! dropping resp %s :/",
				f.filesystemId,
				(*req.Args)["RequestId"].(string),
				resp,
			)
		}
		log.Printf("[run:%s] reading from external requests", f.filesystemId)
	}
}

func (f *fsMachine) pollDirty() error {
	kapi, err := getEtcdKeysApi()
	if err != nil {
		return err
	}
	for {
		if f.filesystem.mounted {
			dirtyDelta, sizeBytes, err := getDirtyDelta(
				f.filesystemId, f.latestSnapshot(),
			)
			if err != nil {
				return err
			}
			if f.dirtyDelta != dirtyDelta || f.sizeBytes != sizeBytes {
				f.dirtyDelta = dirtyDelta
				f.sizeBytes = sizeBytes

				serialized, err := json.Marshal(dirtyInfo{
					Server:     f.state.myNodeId,
					DirtyBytes: dirtyDelta,
					SizeBytes:  sizeBytes,
				})
				if err != nil {
					return err
				}
				_, err = kapi.Set(
					context.Background(),
					fmt.Sprintf(
						"%s/filesystems/dirty/%s", ETCD_PREFIX, f.filesystemId,
					),
					string(serialized),
					nil,
				)
				if err != nil {
					return err
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// return the latest snapshot id, or "" if none exists
func (f *fsMachine) latestSnapshot() string {
	f.snapshotsLock.Lock()
	defer f.snapshotsLock.Unlock()
	if len(f.filesystem.snapshots) > 0 {
		return f.filesystem.snapshots[len(f.filesystem.snapshots)-1].Id
	}
	return ""
}

func (f *fsMachine) getResponseChan(reqId string, e *Event) (chan *Event, error) {
	f.responsesLock.Lock()
	respChan, ok := f.responses[reqId]
	f.responsesLock.Unlock()
	if !ok {
		return nil, fmt.Errorf("No such request id response channel %s", reqId)
	}
	return respChan, nil
}

func (f *fsMachine) updateEtcdAboutSnapshots() error {
	for {
		// attempt to connect to etcd
		kapi, err := getEtcdKeysApi()
		if err != nil {
			return err
		}
		// as soon as we're connected, eagerly: if we know about some
		// snapshots, set them in etcd.
		informed := false
		f.snapshotsLock.Lock()
		if f.filesystem.snapshots != nil {
			informed = true
		}
		f.snapshotsLock.Unlock()

		if informed {
			f.snapshotsLock.Lock()
			serialized, err := json.Marshal(f.filesystem.snapshots)
			if err != nil {
				return err
			}
			f.snapshotsLock.Unlock()
			// since we want atomic rewrites, we can just save the entire
			// snapshot data in a single key, as a json list. this is easier to
			// begin with! although we'll bump into the 1MB request limit in
			// etcd eventually.
			_, err = kapi.Set(
				context.Background(),
				fmt.Sprintf(
					"%s/servers/snapshots/%s/%s", ETCD_PREFIX,
					f.state.myNodeId, f.filesystemId,
				),
				string(serialized),
				nil,
			)
			if err != nil {
				log.Printf(
					"[updateEtcdAboutSnapshots] successfully set new snaps for %s on %s,"+
						" will we hear an echo?",
					f.filesystemId, f.state.myNodeId,
				)
				return err
			}
		}

		// wait until the state machine notifies us that it's changed the
		// snapshots
		_ = <-f.snapshotsModified
		log.Printf("[updateEtcdAboutSnapshots] going 'round the loop")
	}
}

func (f *fsMachine) getCurrentState() string {
	// abusing snapshotsLock here, maybe we should have a separate lock over
	// these fields
	f.snapshotsLock.Lock()
	defer f.snapshotsLock.Unlock()
	return f.currentState
}

func (f *fsMachine) transitionedTo(state string, status string) {
	// abusing snapshotsLock here, maybe we should have a separate lock over
	// these fields
	f.snapshotsLock.Lock()
	defer f.snapshotsLock.Unlock()
	now := time.Now().UnixNano()
	log.Printf(
		"<transition> %s to %s %s (from %s %s, %.2fs ago)",
		f.filesystemId, state, status, f.currentState, f.status,
		float64(now-f.lastTransitionTimestamp)/float64(time.Second),
	)
	f.currentState = state
	f.status = status
	f.lastTransitionTimestamp = now
	f.transitionObserver.Publish("transitions", state)
	// update etcd
	kapi, err := getEtcdKeysApi()
	if err != nil {
		log.Printf("error connecting to etcd while trying to update states: %s", err)
		return
	}
	update := map[string]string{
		"state": state, "status": status,
	}
	serialized, err := json.Marshal(update)
	if err != nil {
		log.Printf("cannot serialize %s: %s", update, err)
		return
	}
	_, err = kapi.Set(
		context.Background(),
		// .../:server/:filesystem = {"state": "inactive", "status": "pulling..."}
		fmt.Sprintf(
			"%s/servers/states/%s/%s",
			ETCD_PREFIX, f.state.myNodeId, f.filesystemId,
		),
		string(serialized),
		nil,
	)
	if err != nil {
		log.Printf("error updating etcd: %s", update, err)
		return
	}
}

// state functions
// invariant: whenever a state function receives on the events channel, it
// should respond with a response event, even in an error case.

func handoffState(f *fsMachine) stateFn {
	f.transitionedTo("handoff", "starting...")
	// I am a master, trying to move this filesystem to a slave.
	// I got put into this state in response to a "move" event on f.requests,
	// so it's my responsibility to put something onto f.responses, because
	// there'll be someone out there listening for my response...
	// I assume that previous states stopped any containers that were running
	// on this filesystem, so the filesystem is quiescent.
	// TODO stop any containers being able to get started here.
	target := (*f.handoffRequest.Args)["target"].(string)
	log.Printf("Found target node %s", target)

	// subscribe for snapshot updates before we start sending, in case of races...
	newSnapsChan := make(chan interface{})
	f.newSnapsOnServers.Subscribe(target, newSnapsChan)
	defer f.newSnapsOnServers.Unsubscribe(target, newSnapsChan)

	// unmount the filesystem immediately, so that the filesystem doesn't get
	// dirtied by being unmounted
	event, _ := f.unmount()
	if event.Name != "unmounted" {
		log.Printf("unexpected response to unmount attempt: %s", event)
		return backoffState
	}

	// XXX if we error out of handoffState, we'll end up in an infinite loop if
	// we don't re-mount the filesystem. see comment in backoffState for
	// possible fix.

	// take a snapshot and wait for it to arrive on the target
	response, _ := f.snapshot(&Event{
		Name: "snapshot",
		Args: &EventArgs{"metadata": metadata{
			"author": "system",
			"message": fmt.Sprintf(
				"Automatic snapshot during migration from %s to %s.",
				f.state.myNodeId, target,
			)},
		},
	})
	f.transitionedTo("handoff", fmt.Sprintf("got snapshot response %s", response))
	if response.Name != "snapshotted" {
		// error - bail
		f.innerResponses <- response
		return backoffState
	}
	slaveUpToDate := false

waitingForSlaveSnapshot:
	for !slaveUpToDate {
		// ok, so snapshot succeeded. wait for it to be replicated to the
		// target node (it should be, naturally because currently we replicate
		// everything everywhere)
		f.transitionedTo("handoff", fmt.Sprintf("calling snapshotsFor %s", target))
		slaveSnapshots, err := f.state.snapshotsFor(target, f.filesystemId)
		f.transitionedTo(
			"handoff",
			fmt.Sprintf("done calling snapshotsFor %s: %s", target, err),
		)
		if err != nil {
			// Let's assume that no record of snapshots on a node means no
			// filesystem there. If we're wrong and there /is/ a filesystem
			// there with no snapshots, we won't be able to receive into it.
			// But this shouldn't happen because you can only create a
			// filesystem if you can write atomically to etcd, claiming its
			// name for yourself.
			log.Printf(
				"Unable to find target snaps for %s on %s, assuming there are none and proceeding...",
				f.filesystemId, target,
			)
		}
		f.transitionedTo(
			"handoff",
			fmt.Sprintf("finding own snaps for move to %s", target),
		)

		// information about our new snapshot probably hasn't roundtripped
		// through etcd yet, so use our definitive knowledge about our local
		// state...
		f.snapshotsLock.Lock()
		snaps := f.filesystem.snapshots
		f.snapshotsLock.Unlock()

		f.transitionedTo(
			"handoff",
			fmt.Sprintf("done finding own snaps for move to %s", target),
		)

		apply, err := canApply(snaps, pointers(slaveSnapshots))
		f.transitionedTo(
			"handoff",
			fmt.Sprintf("canApply returned %s, %s", apply, err),
		)
		if err != nil {
			switch err.(type) {
			case *ToSnapsUpToDate:
				log.Printf("Found ToSnapsUpToDate, setting slaveUpToDate for %s", f.filesystemId)
				slaveUpToDate = true
				break waitingForSlaveSnapshot
			}
		} else {
			err = fmt.Errorf(
				"ff update of %s for %s to %s was possible, can't move yet, retrying...",
				f.filesystemId, f.state.myNodeId, target,
			)
		}
		if !slaveUpToDate {
			log.Printf(
				"Not proceeding with migration yet for %s from %s to %s because %s, waiting for new snaps...",
				f.filesystemId, f.state.myNodeId, target, err,
			)
		}

		// TODO timeout, or liveness check on replication
		log.Printf("About to read from newSnapsChan(%s) we created earlier", target)

		// say no to everything right now, but don't clog up requests
		gotSnaps := false
		for !gotSnaps {
			select {
			case e := <-f.innerRequests:
				log.Printf("rejecting all %s", e)
				f.innerResponses <- &Event{"busy-handoff", &EventArgs{}}
			case _ = <-newSnapsChan:
				// TODO check that the latest snap is the one we expected
				gotSnaps = true
				log.Printf("Got new snaps of %s on %s", f.filesystemId, target)
				// carry on
			}
		}
	}
	// cool, fs is quiesced and latest snap is on target. switch!

	kapi, err := getEtcdKeysApi()
	if err != nil {
		f.innerResponses <- &Event{Name: "failed-to-connect-to-etcd"}
	}
	_, err = kapi.Set(
		context.Background(),
		fmt.Sprintf(
			"%s/filesystems/masters/%s", ETCD_PREFIX, f.filesystemId,
		),
		target,
		// only modify current master if I am indeed still the master
		&client.SetOptions{PrevValue: f.state.myNodeId},
	)
	f.innerResponses <- &Event{Name: "moved"}
	return inactiveState
}

func (f *fsMachine) unmount() (responseEvent *Event, nextState stateFn) {
	out, err := exec.Command("umount", mnt(f.filesystemId)).CombinedOutput()
	if err != nil {
		log.Printf("%v while trying to unmount %s", err, fq(f.filesystemId))
		return &Event{
			Name: "failed-unmount",
			Args: &EventArgs{"err": err, "combined-output": string(out)},
		}, backoffState
	}
	f.filesystem.mounted = false
	return &Event{Name: "unmounted"}, inactiveState
}

func (f *fsMachine) snapshot(e *Event) (responseEvent *Event, nextState stateFn) {
	var meta metadata
	if val, ok := (*e.Args)["metadata"]; ok {
		meta = castToMetadata(val)
	} else {
		meta = metadata{}
	}
	meta["timestamp"] = fmt.Sprintf("%d", time.Now().UnixNano())
	metadataEncoded, err := encodeMetadata(meta)
	if err != nil {
		return &Event{
			Name: "failed-metadata-encode", Args: &EventArgs{"err": err},
		}, backoffState
	}
	id, err := uuid.NewV4()
	if err != nil {
		return &Event{
			Name: "failed-uuid", Args: &EventArgs{"err": err},
		}, backoffState
	}
	snapshotId := id.String()
	args := []string{"snapshot"}
	args = append(args, metadataEncoded...)
	args = append(args, fq(f.filesystemId)+"@"+snapshotId)
	out, err := exec.Command(ZFS, args...).CombinedOutput()
	log.Printf("[snapshot] Attempting: zfs %s", args)
	if err != nil {
		log.Printf("[snapshot] %v while trying to snapshot %s (%s)", err, fq(f.filesystemId), args)
		return &Event{
			Name: "failed-snapshot",
			Args: &EventArgs{"err": err, "combined-output": string(out)},
		}, backoffState
	}
	list, err := exec.Command(ZFS, "list", fq(f.filesystemId)+"@"+snapshotId).CombinedOutput()
	if err != nil {
		log.Printf("[snapshot] %v while trying to list snapshot %s (%s)", err, fq(f.filesystemId), args)
		return &Event{
			Name: "failed-snapshot",
			Args: &EventArgs{"err": err, "combined-output": string(out)},
		}, backoffState

	}
	log.Printf("[snapshot] listed snapshot: '%q'", strconv.Quote(string(list)))
	f.snapshotsLock.Lock()
	log.Printf("[snapshot] Succeeded snapshotting (out: '%s'), saving: %s", out, &snapshot{Id: snapshotId, Metadata: &meta})
	f.filesystem.snapshots = append(f.filesystem.snapshots,
		&snapshot{Id: snapshotId, Metadata: &meta})
	f.snapshotsLock.Unlock()
	f.snapshotsModified <- true
	return &Event{Name: "snapshotted"}, activeState
}

// find the user-facing name of a given filesystem id. if we're a branch
// (clone), return the name of our parent filesystem.
func (f *fsMachine) name() (string, error) {
	tlf, _, err := f.state.registry.LookupFilesystemId(f.filesystemId)
	return tlf.TopLevelVolume.Name, err
}

func (f *fsMachine) containersRunning() ([]DockerContainer, error) {
	f.state.containersLock.Lock()
	defer f.state.containersLock.Unlock()
	name, err := f.name()
	if err != nil {
		return []DockerContainer{}, err
	}
	return f.state.containers.Related(name)
}

func (f *fsMachine) stopContainers() error {
	f.state.containersLock.Lock()
	defer f.state.containersLock.Unlock()
	name, err := f.name()
	if err != nil {
		return err
	}
	return f.state.containers.Stop(name)
}

func (f *fsMachine) startContainers() error {
	f.state.containersLock.Lock()
	defer f.state.containersLock.Unlock()
	name, err := f.name()
	if err != nil {
		return err
	}
	return f.state.containers.Start(name)
}

func activeState(f *fsMachine) stateFn {
	f.transitionedTo("active", "waiting")
	log.Printf("entering active state for %s", f.filesystemId)
	select {
	case e := <-f.innerRequests:
		if e.Name == "transfer" {

			// TODO dedupe
			transferRequest, err := transferRequestify((*e.Args)["Transfer"])
			if err != nil {
				f.innerResponses <- &Event{
					Name: "cant-cast-transfer-request",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.lastTransferRequest = transferRequest
			transferRequestId, ok := (*e.Args)["RequestId"].(string)
			if !ok {
				f.innerResponses <- &Event{
					Name: "cant-cast-transfer-requestid",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.lastTransferRequestId = transferRequestId

			log.Printf("GOT TRANSFER REQUEST %s", f.lastTransferRequest)
			if f.lastTransferRequest.Direction == "push" {
				return pushInitiatorState
			} else if f.lastTransferRequest.Direction == "pull" {
				return pullInitiatorState
			}
		} else if e.Name == "peer-transfer" {

			// TODO dedupe
			transferRequest, err := transferRequestify((*e.Args)["Transfer"])
			if err != nil {
				f.innerResponses <- &Event{
					Name: "cant-cast-transfer-request",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.lastTransferRequest = transferRequest
			transferRequestId, ok := (*e.Args)["RequestId"].(string)
			if !ok {
				f.innerResponses <- &Event{
					Name: "cant-cast-transfer-requestid",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.lastTransferRequestId = transferRequestId

			log.Printf("GOT PEER TRANSFER REQUEST %s", f.lastTransferRequest)
			if f.lastTransferRequest.Direction == "push" {
				return pushPeerState
			} else if f.lastTransferRequest.Direction == "pull" {
				return pullPeerState
			}
		} else if e.Name == "move" {
			// move straight into a state which doesn't allow us to take
			// snapshots or do rollbacks
			// refuse to move if we have any containers running
			containers, err := f.containersRunning()
			if err != nil {
				log.Printf(
					"Can't move filesystem while we can't list whether containers are using it",
				)
				f.innerResponses <- &Event{
					Name: "error-listing-containers-during-move",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			if len(containers) > 0 {
				log.Printf("Can't move filesystem while containers are using it")
				f.innerResponses <- &Event{
					Name: "cannot-move-while-containers-running",
					Args: &EventArgs{"containers": containers},
				}
				return backoffState
			}
			f.handoffRequest = e
			return handoffState
		} else if e.Name == "snapshot" {
			response, state := f.snapshot(e)
			f.innerResponses <- response
			return state
		} else if e.Name == "rollback" {
			// roll back to given snapshot
			rollbackTo := (*e.Args)["rollbackTo"].(string)
			// TODO also roll back slaves (i.e., support doing this in unmounted state)
			sliceIndex := -1
			for i, snapshot := range f.filesystem.snapshots {
				if snapshot.Id == rollbackTo {
					// the first *deleted* snapshot will be the one *after*
					// rollbackTo
					sliceIndex = i + 1
				}
			}
			// XXX This is broken for pinned branches right now
			err := f.stopContainers()
			defer func() {
				err := f.startContainers()
				if err != nil {
					log.Printf(
						"[activeState] unable to start containers in deferred func: %s",
						err,
					)
				}
			}()
			if err != nil {
				log.Printf(
					"%v while trying to stop containers during rollback %s",
					err, fq(f.filesystemId),
				)
				f.innerResponses <- &Event{
					Name: "failed-stop-containers-during-rollback",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			out, err := exec.Command(ZFS, "rollback",
				"-r", fq(f.filesystemId)+"@"+rollbackTo).CombinedOutput()
			if err != nil {
				log.Printf("%v while trying to rollback %s", err, fq(f.filesystemId))
				f.innerResponses <- &Event{
					Name: "failed-rollback",
					Args: &EventArgs{"err": err, "combined-output": string(out)},
				}
				return backoffState
			}
			if sliceIndex > 0 {
				log.Printf("found index %d", sliceIndex)
				log.Printf("snapshots before %s", f.filesystem.snapshots)
				f.snapshotsLock.Lock()
				f.filesystem.snapshots = f.filesystem.snapshots[:sliceIndex]
				f.snapshotsLock.Unlock()
				f.snapshotsModified <- true
				log.Printf("snapshots after %s", f.filesystem.snapshots)
			} else {
				f.innerResponses <- &Event{
					Name: "no-such-snapshot",
				}
			}
			err = f.startContainers()
			if err != nil {
				log.Printf(
					"%v while trying to start containers during rollback %s",
					err, fq(f.filesystemId),
				)
				f.innerResponses <- &Event{
					Name: "failed-start-containers-during-rollback",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.innerResponses <- &Event{
				Name: "rolled-back",
			}
			return activeState
		} else if e.Name == "clone" {
			// clone a new filesystem from the given snapshot, then spin off a
			// new fsMachine for it.

			/*
				"topLevelFilesystemId": topLevelFilesystemId,
				"originFilesystemId":   originFilesystemId,
				"originSnapshotId":     args.SourceSnapshotId,
				"newBranchName":        args.NewBranchName,
			*/

			topLevelFilesystemId := (*e.Args)["topLevelFilesystemId"].(string)
			originFilesystemId := (*e.Args)["originFilesystemId"].(string)
			originSnapshotId := (*e.Args)["originSnapshotId"].(string)
			newBranchName := (*e.Args)["newBranchName"].(string)

			uuid, err := uuid.NewV4()
			if err != nil {
				f.innerResponses <- &Event{
					Name: "failed-uuid", Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			newCloneFilesystemId := uuid.String()

			// RegisterClone(name string, topLevelFilesystemId string, clone Clone)
			err = f.state.registry.RegisterClone(
				newBranchName, topLevelFilesystemId,
				Clone{
					newCloneFilesystemId,
					Origin{
						originFilesystemId, originSnapshotId,
					},
				},
			)
			if err != nil {
				f.innerResponses <- &Event{
					Name: "failed-clone-registration", Args: &EventArgs{"err": err},
				}
				return backoffState
			}

			out, err := exec.Command(
				ZFS, "clone",
				fq(f.filesystemId)+"@"+originSnapshotId,
				fq(newCloneFilesystemId),
			).CombinedOutput()
			if err != nil {
				log.Printf("%v while trying to clone %s", err, fq(f.filesystemId))
				f.innerResponses <- &Event{
					Name: "failed-clone",
					Args: &EventArgs{"err": err, "combined-output": string(out)},
				}
				return backoffState
			}
			// spin off a state machine
			f.state.initFilesystemMachine(newCloneFilesystemId)
			kapi, err := getEtcdKeysApi()
			if err != nil {
				f.innerResponses <- &Event{
					Name: "failed-get-etcd",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			// claim the clone as mine, so that it can be mounted here
			_, err = kapi.Set(
				context.Background(),
				fmt.Sprintf(
					"%s/filesystems/masters/%s", ETCD_PREFIX, newCloneFilesystemId,
				),
				f.state.myNodeId,
				// only modify current master if this is a new filesystem id
				&client.SetOptions{PrevExist: client.PrevNoExist},
			)
			if err != nil {
				f.innerResponses <- &Event{
					Name: "failed-make-cloner-master",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.innerResponses <- &Event{
				Name: "cloned",
				Args: &EventArgs{},
			}
			return activeState
		} else if e.Name == "unmount" {
			// fail if any containers running
			containers, err := f.containersRunning()
			if err != nil {
				log.Printf("Can't unmount filesystem while containers are using it")
				f.innerResponses <- &Event{
					Name: "error-listing-containers-during-unmount",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			if len(containers) > 0 {
				f.innerResponses <- &Event{
					Name: "cannot-unmount-while-running-containers",
					Args: &EventArgs{"containers": containers},
				}
				return backoffState
			}
			response, state := f.unmount()
			f.innerResponses <- response
			return state
		} else {
			f.innerResponses <- &Event{
				Name: "unhandled",
				Args: &EventArgs{"current-state": f.currentState, "event": e},
			}
			log.Printf("unhandled event %s while in missingState", e)
		}
	}
	// something unknown happened, go and check the state of the system after a
	// short timeout to avoid busylooping
	return backoffState
}

// probably the wrong way to do it
func pointers(snapshots []snapshot) []*snapshot {
	newList := []*snapshot{}
	for _, snap := range snapshots {
		s := &snapshot{}
		*s = snap
		newList = append(newList, s)
	}
	return newList
}

func (f *fsMachine) mount() (responseEvent *Event, nextState stateFn) {
	out, err := exec.Command(
		"mkdir", "-p", mnt(f.filesystemId)).CombinedOutput()
	if err != nil {
		log.Printf("%v while trying to mkdir mountpoint %s", err, fq(f.filesystemId))
		return &Event{
			Name: "failed-mkdir-mountpoint",
			Args: &EventArgs{"err": err, "combined-output": string(out)},
		}, backoffState
	}
	out, err = exec.Command("mount.zfs",
		fq(f.filesystemId), mnt(f.filesystemId)).CombinedOutput()
	if err != nil {
		log.Printf("%v while trying to mount %s", err, fq(f.filesystemId))
		return &Event{
			Name: "failed-mount",
			Args: &EventArgs{"err": err, "combined-output": string(out)},
		}, backoffState
	}
	// trust that zero exit codes from mkdir && mount.zfs means
	// that it worked and that the filesystem now exists and is
	// mounted
	f.snapshotsLock.Lock()
	f.filesystem.exists = true // needed in create case
	f.filesystem.mounted = true
	f.snapshotsLock.Unlock()
	return &Event{Name: "mounted", Args: &EventArgs{}}, activeState
}

func inactiveState(f *fsMachine) stateFn {
	f.transitionedTo("inactive", "waiting")
	log.Printf("entering inactive state for %s", f.filesystemId)

	handleEvent := func(e *Event) (bool, stateFn) {
		if e.Name == "mount" {
			f.transitionedTo("inactive", "mounting")
			event, nextState := f.mount()
			f.innerResponses <- event
			return true, nextState
		} else {
			f.innerResponses <- &Event{
				Name: "unhandled",
				Args: &EventArgs{"current-state": f.currentState, "event": e},
			}
			log.Printf("unhandled event %s while in inactiveState", e)
		}
		return false, nil
	}

	// ensure that if there's an event on the channel which a receive was
	// cancelled in order to process, that we process that immediately before
	// going back into receive. do this with an asynchronous read before
	// checking going back into receive...
	// TODO test this behaviour

	select {
	case e := <-f.innerRequests:
		doTransition, nextState := handleEvent(e)
		if doTransition {
			return nextState
		}
	default:
		// carry on
	}

	if f.attemptReceive() {
		return receivingState
	}

	newSnapsOnMaster := make(chan interface{})
	f.state.newSnapsOnMaster.Subscribe(f.filesystemId, newSnapsOnMaster)
	defer f.state.newSnapsOnMaster.Unsubscribe(f.filesystemId, newSnapsOnMaster)

	select {
	case _ = <-newSnapsOnMaster:
		return receivingState
	case e := <-f.innerRequests:
		doTransition, nextState := handleEvent(e)
		if doTransition {
			return nextState
		}
	}
	return backoffState
}

func (f *fsMachine) plausibleSnapRange() (*snapshotRange, error) {
	// get all snapshots for the given filesystem on the current master, and
	// then start a pull if we need to
	snapshots, err := f.state.snapshotsForCurrentMaster(f.filesystemId)
	if err != nil {
		return nil, err
	}

	f.snapshotsLock.Lock()
	snapRange, err := canApply(pointers(snapshots), f.filesystem.snapshots)
	f.snapshotsLock.Unlock()

	return snapRange, err
}

func (f *fsMachine) attemptReceive() bool {
	// Check whether there are any pull-able snaps of this filesystem on its
	// current master

	_, err := f.plausibleSnapRange()

	// The non-error case plus all error cases except the ones below
	// indicate that some substantial action (receive, clone-and-rollback,
	// etc) is possible in receivingState, in those cases let's go there and
	// make progress.
	if err != nil {
		switch err := err.(type) {
		case *ToSnapsUpToDate:
			// no action, we're up-to-date
			return false
		case *NoFromSnaps:
			// no snaps; can't replicate yet
			return false
		default:
			// some other error
			log.Printf("Not attempting to receive %s because: %s", f.filesystemId, err)
			return false
		}
	} else {
		// non-error canApply implies clean fastforward apply is possible
		return true
	}
}

func transferRequestify(in interface{}) (TransferRequest, error) {
	typed, ok := in.(map[string]interface{})
	if !ok {
		log.Printf("[transferRequestify] Unable to cast %s to map[string]interface{}", in)
		return TransferRequest{}, fmt.Errorf(
			"Unable to cast %s to map[string]interface{}", in,
		)
	}
	return TransferRequest{
		Peer:                 typed["Peer"].(string),
		User:                 typed["User"].(string),
		ApiKey:               typed["ApiKey"].(string),
		Direction:            typed["Direction"].(string),
		LocalFilesystemName:  typed["LocalFilesystemName"].(string),
		LocalCloneName:       typed["LocalCloneName"].(string),
		RemoteFilesystemName: typed["RemoteFilesystemName"].(string),
		RemoteCloneName:      typed["RemoteCloneName"].(string),
		TargetSnapshot:       typed["TargetSnapshot"].(string),
	}, nil
}

// either missing because you're about to be locally created or because the
// filesystem exists somewhere else in the cluster
func missingState(f *fsMachine) stateFn {
	f.transitionedTo("missing", "waiting")
	log.Printf("entering missing state for %s", f.filesystemId)

	if f.attemptReceive() {
		return receivingState
	}

	newSnapsOnMaster := make(chan interface{})
	f.state.newSnapsOnMaster.Subscribe(f.filesystemId, newSnapsOnMaster)
	defer f.state.newSnapsOnMaster.Unsubscribe(f.filesystemId, newSnapsOnMaster)

	select {
	case _ = <-newSnapsOnMaster:
		return receivingState
	case e := <-f.innerRequests:
		if e.Name == "transfer" {
			log.Printf("GOT TRANSFER REQUEST (while missing) %s", e.Args)

			// TODO dedupe
			transferRequest, err := transferRequestify((*e.Args)["Transfer"])
			if err != nil {
				f.innerResponses <- &Event{
					Name: "cant-cast-transfer-request",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.lastTransferRequest = transferRequest
			transferRequestId, ok := (*e.Args)["RequestId"].(string)
			if !ok {
				f.innerResponses <- &Event{
					Name: "cant-cast-transfer-requestid",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.lastTransferRequestId = transferRequestId

			if f.lastTransferRequest.Direction == "push" {
				// Can't push when we're missing.
				f.innerResponses <- &Event{
					Name: "cant-push-while-missing",
					Args: &EventArgs{"request": e, "node": f.state.myNodeId},
				}
				return backoffState
			} else if f.lastTransferRequest.Direction == "pull" {
				return pullInitiatorState
			}
		} else if e.Name == "peer-transfer" {
			// A transfer has been registered. Try to go into the appropriate
			// state.

			// TODO dedupe
			transferRequest, err := transferRequestify((*e.Args)["Transfer"])
			if err != nil {
				f.innerResponses <- &Event{
					Name: "cant-cast-transfer-request",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.lastTransferRequest = transferRequest
			transferRequestId, ok := (*e.Args)["RequestId"].(string)
			if !ok {
				f.innerResponses <- &Event{
					Name: "cant-cast-transfer-requestid",
					Args: &EventArgs{"err": err},
				}
				return backoffState
			}
			f.lastTransferRequestId = transferRequestId

			if f.lastTransferRequest.Direction == "pull" {
				// Can't provide for an initiator trying to pull when we're missing.
				f.innerResponses <- &Event{
					Name: "cant-provide-pull-while-missing",
					Args: &EventArgs{"request": e, "node": f.state.myNodeId},
				}
				return backoffState
			} else if f.lastTransferRequest.Direction == "push" {
				log.Printf("GOT PEER TRANSFER REQUEST FROM MISSING %s", f.lastTransferRequest)
				return pushPeerState
			}
		} else if e.Name == "create" {
			f.transitionedTo("missing", "creating")
			// ah - we are going to be created on this node, rather than
			// received into from a master...
			log.Printf("%s %s %s", ZFS, "create", fq(f.filesystemId))
			out, err := exec.Command(ZFS, "create", fq(f.filesystemId)).CombinedOutput()
			if err != nil {
				log.Printf("%v while trying to create %s", err, fq(f.filesystemId))
				f.innerResponses <- &Event{
					Name: "failed-create",
					Args: &EventArgs{"err": err, "combined-output": string(out)},
				}
				return backoffState
			}
			responseEvent, nextState := f.mount()
			if responseEvent.Name == "mounted" {
				f.innerResponses <- &Event{Name: "created"}
				return activeState
			} else {
				f.innerResponses <- responseEvent
				return nextState
			}
		} else {
			f.innerResponses <- &Event{
				Name: "unhandled",
				Args: &EventArgs{"current-state": f.currentState, "event": e},
			}
			log.Printf("unhandled event %s while in missingState", e)
		}
	}
	// something unknown happened, go and check the state of the system after a
	// short timeout to avoid busylooping
	return backoffState
}

func backoffState(f *fsMachine) stateFn {
	f.transitionedTo("backoff", "pausing")
	log.Printf("entering backoff state for %s", f.filesystemId)
	// TODO if we know that we're supposed to be mounted or unmounted, based on
	// etcd state, actually put us back into the required state rather than
	// just passively going back into discovering... or maybe, do that in
	// discoveringState?
	time.Sleep(time.Second)
	return discoveringState
}

func (f *fsMachine) discover() error {
	// discover system state synchronously
	filesystem, err := discoverSystem(f.filesystemId)
	if err != nil {
		return err
	}
	f.snapshotsLock.Lock()
	f.filesystem = filesystem
	f.snapshotsLock.Unlock()

	// quite probably we just learned about some snapshots we didn't know about
	// before
	f.snapshotsModified <- true
	// we won't hear an "echo" from etcd about our own snapshots, so
	// synchronously update our own "global" cache about them, too, notifying
	// any observers in the process.
	// XXX this _might_ break the fact that handoff doesn't check what snapshot
	// it's notified about.
	f.snapshotsLock.Lock()
	snaps := filesystem.snapshots
	f.snapshotsLock.Unlock()

	// []*snapshot => []snapshot, gah
	snapsAlternate := []snapshot{}
	for _, snap := range snaps {
		snapsAlternate = append(snapsAlternate, *snap)
	}

	return f.state.updateSnapshotsFromKnownState(
		f.state.myNodeId, f.filesystemId, &snapsAlternate,
	)
}

func discoveringState(f *fsMachine) stateFn {
	f.transitionedTo("discovering", "loading")
	log.Printf("entering discovering state for %s", f.filesystemId)

	err := f.discover()
	if err != nil {
		log.Printf("%v while discovering state", err)
		return backoffState
	}

	if !f.filesystem.exists {
		return missingState
	} else {
		if f.filesystem.mounted {
			return activeState
		} else {
			return inactiveState
		}
	}
}

// attempt to pull some snapshots from the master, based on some hint that it
// might be possible now
func receivingState(f *fsMachine) stateFn {
	f.transitionedTo("receiving", "calculating")
	log.Printf("entering receiving state for %s", f.filesystemId)
	snapRange, err := f.plausibleSnapRange()

	// by judiciously reading from f.innerRequests, we implicitly take a lock on not
	// changing mount state until we finish receiving or an attempt to change
	// mount state results in us being cancelled and finish cancelling

	if err != nil {
		switch err := err.(type) {
		// TODO should the following 'discoveringState's be 'backoffState's?
		case *ToSnapsUpToDate:
			log.Printf("receivingState: ToSnapsUpToDate %s got %s", f.filesystemId, err)
			// this is fine, we're up-to-date
			return discoveringState
		case *NoFromSnaps:
			log.Printf("receivingState: NoFromSnaps %s got %s", f.filesystemId, err)
			// this is fine, no snaps; can't replicate yet, but will
			return discoveringState
		case *ToSnapsAhead:
			log.Printf("receivingState: ToSnapsAhead %s got %s", f.filesystemId, err)
			// erk, slave is ahead of master
			// TODO: create a new local clone (branch), then roll back to
			// err.latestCommonSnapshot (except, you can't roll back a snapshot
			// that a clone depends on without promoting the clone... hmmm)
			return discoveringState
		case *ToSnapsDiverged:
			log.Printf("receivingState: ToSnapsDiverged %s got %s", f.filesystemId, err)
			// erk, slave has diverged from master
			// TODO: create a new local clone (branch), then roll back to
			// err.latestCommonSnapshot (except, you can't roll back a snapshot
			// that a clone depends on without promoting the clone... hmmm)
			return discoveringState
		case *NoCommonSnapshots:
			log.Printf("receivingState: NoCommonSnapshots %s got %s", f.filesystemId, err)
			// erk, no common snapshots between master and slave
			// TODO: create a new local clone (branch), then delete the current
			// filesystem to enable replication to continue
			return discoveringState
		default:
			log.Printf("receivingState: default error handler %s got %s", f.filesystemId, err)
			return discoveringState
		}
	}

	var fromSnap string
	if snapRange.fromSnap == nil {
		fromSnap = "START"
		// it's possible this is the first snapshot for a clone. check, and if
		// it is, attempt to generate a replication stream from the clone's
		// origin. it might be the case that the clone's origin doesn't exist
		// here, in which case the apply will fail.
		clone, err := f.state.registry.LookupCloneById(f.filesystemId)
		if err != nil {
			switch err := err.(type) {
			case NoSuchClone:
				// Normal case for non-clone filesystems, continue.
			default:
				log.Printf("Error trying to lookup clone by id: %s", err)
				return backoffState
			}
		} else {
			// Found a clone, let's base our pull on it
			fromSnap = fmt.Sprintf(
				"%s@%s", clone.Origin.FilesystemId, clone.Origin.SnapshotId,
			)
		}
	} else {
		fromSnap = snapRange.fromSnap.Id
	}

	addresses := f.state.addressesFor(
		f.state.masterFor(f.filesystemId),
	)
	if len(addresses) == 0 {
		log.Printf("No known address for current master of %s", f.filesystemId)
		return backoffState
	}
	// XXX hack, IPv4 happens to come before IPv6 and happens to be routeable
	// on my network (whereas IPv6 isn't), but this depends on the enumeration
	// order of network cards in servers :/
	// TODO we should really attempt each address in turn until we find one
	// that works.
	peerAddress := addresses[0]

	pw, err := getPassword("admin")
	if err != nil {
		log.Printf("Attempting to pull %s got %s", f.filesystemId, err)
		return backoffState
	}
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"http://%s:6969/filesystems/%s/%s/%s", peerAddress,
			f.filesystemId, fromSnap, snapRange.toSnap.Id,
		),
		nil,
	)
	if err != nil {
		log.Printf("Attempting to pull %s got %s", f.filesystemId, err)
		return backoffState
	}
	req.SetBasicAuth("admin", pw)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Attempting to pull %s got %s", f.filesystemId, err)
		return backoffState
	}
	log.Printf(
		"Debug: curl -u admin:[pw] http://%s:6969/filesystems/%s/%s/%s",
		peerAddress, f.filesystemId, fromSnap, snapRange.toSnap.Id,
	)

	f.transitionedTo("receiving", "starting")
	cmd := exec.Command("zfs", "recv", fq(f.filesystemId))
	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()
	defer pipeWriter.Close()

	cmd.Stdin = pipeReader
	cmd.Stdout = getLogfile("zfs-recv-stdout")
	cmd.Stderr = getLogfile("zfs-recv-stderr")

	finished := make(chan bool)

	go pipe(
		resp.Body, fmt.Sprintf("http response body for %s", f.filesystemId),
		pipeWriter, "stdin of zfs recv",
		finished,
		f.innerRequests,
		// put the event back on the channel in the cancellation case
		func(e *Event, c chan *Event) { c <- e },
		func(bytes int64, t int64) {
			f.transitionedTo("receiving",
				fmt.Sprintf(
					"transferred %.2fMiB in %.2fs (%.2fMiB/s)...",
					// bytes => mebibytes       nanoseconds => seconds
					float64(bytes)/(1024*1024), float64(t)/(1000*1000*1000),
					// mib/sec
					(float64(bytes)/(1024*1024))/(float64(t)/(1000*1000*1000)),
				),
			)
		},
		"decompress",
	)

	log.Printf("[pull] about to start consuming prelude on %v", pipeReader)
	prelude, err := consumePrelude(pipeReader)
	if err != nil {
		return backoffState
	}
	log.Printf("[pull] Got prelude %v", prelude)

	err = cmd.Run()
	f.transitionedTo("receiving", "finished zfs recv")
	pipeReader.Close()
	pipeWriter.Close()
	_ = <-finished
	f.transitionedTo("receiving", "finished pipe")

	if err != nil {
		log.Printf(
			"Got error %s when running zfs recv for %s, check zfs-recv-stderr.log",
			err, f.filesystemId,
		)
		return backoffState
	} else {
		log.Printf("Successfully received %s => %s for %s", fromSnap, snapRange.toSnap.Id)
	}
	log.Printf("[pull] about to start applying prelude on %v", pipeReader)
	err = applyPrelude(prelude, fq(f.filesystemId))
	if err != nil {
		return backoffState
	}
	return discoveringState
}

func updatePollResult(transferRequestId string, pollResult TransferPollResult) error {
	log.Printf(
		"[updatePollResult] attempting to update poll result for %s: %s",
		transferRequestId, pollResult,
	)
	kapi, err := getEtcdKeysApi()
	if err != nil {
		return err
	}
	serialized, err := json.Marshal(pollResult)
	if err != nil {
		return err
	}
	log.Printf(
		"[updatePollResult] => %s, serialized: %s",
		fmt.Sprintf(
			"%s/filesystems/transfers/%s",
			ETCD_PREFIX,
			transferRequestId,
		),
		string(serialized),
	)
	_, err = kapi.Set(
		context.Background(),
		fmt.Sprintf("%s/filesystems/transfers/%s", ETCD_PREFIX, transferRequestId),
		string(serialized),
		nil,
	)
	log.Printf("[updatePollResult] err: %s", err)
	return err
}

func TransferPollResultFromTransferRequest(
	transferRequestId string,
	transferRequest TransferRequest,
	nodeId string,
	index, total int,
	status string,
) TransferPollResult {
	return TransferPollResult{
		TransferRequestId: transferRequestId,
		Peer:              transferRequest.Peer,
		User:              transferRequest.User,
		ApiKey:            transferRequest.ApiKey,
		Direction:         transferRequest.Direction,

		LocalFilesystemName:  transferRequest.LocalFilesystemName,
		LocalCloneName:       transferRequest.LocalCloneName,
		RemoteFilesystemName: transferRequest.RemoteFilesystemName,
		RemoteCloneName:      transferRequest.RemoteCloneName,

		// XXX filesystemId varies over the lifetime of a transferRequestId...
		// this is certainly a hack, and may be problematic. in particular, it
		// may result in different clones being pushed to different hosts, in
		// the case of a multi-host target cluster, possibly...
		FilesystemId:    "",
		InitiatorNodeId: nodeId,
		// XXX re-inventing a wheel here? Maybe we can just use the state
		// "status" fields for this? We're using that already for inter-cluster
		// replication.
		Index:  index,
		Total:  total,
		Status: status,
	}
}

func pushInitiatorState(f *fsMachine) stateFn {
	// Deduce the latest snapshot in
	// f.lastTransferRequest.LocalFilesystemName:LocalCloneName
	// and try a few times to get it onto the target node.
	f.transitionedTo("pushInitiatorState", "requesting")
	// Set /filesystems/transfers/:transferId = TransferPollResult{...}
	transferRequest := f.lastTransferRequest
	transferRequestId := f.lastTransferRequestId
	path, err := f.state.registry.deducePathToTopLevelFilesystem(
		transferRequest.LocalFilesystemName, transferRequest.LocalCloneName,
	)
	if err != nil {
		f.innerResponses <- &Event{
			Name: "cant-calculate-path-to-snapshot",
			Args: &EventArgs{"err": err},
		}
		return backoffState
	}

	pollResult := TransferPollResultFromTransferRequest(
		transferRequestId, transferRequest, f.state.myNodeId,
		1, 1+len(path.Clones), "syncing metadata",
	)
	f.lastPollResult = &pollResult

	err = updatePollResult(transferRequestId, pollResult)
	if err != nil {
		f.innerResponses <- &Event{
			Name: "push-initiator-cant-write-to-etcd",
			Args: &EventArgs{"err": err},
		}
		return backoffState
	}
	// Also RPC to remote cluster to set up a similar record there.
	// TODO retries
	client := NewJsonRpcClient(
		transferRequest.User,
		transferRequest.Peer,
		transferRequest.ApiKey,
	)

	// TODO should we wait for the remote to ack that it's gone into the right state?

	// retryPush takes filesystem id to push, and final snapshot id (or ""
	// for "up to latest")

	// TODO tidy up argument passing here.
	responseEvent, nextState := f.applyPath(path, func(f *fsMachine,
		fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
		transferRequestId string, pollResult *TransferPollResult,
		client *JsonRpcClient, transferRequest *TransferRequest,
	) (*Event, stateFn) {
		return f.retryPush(
			fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId,
			transferRequestId, pollResult, client, transferRequest,
		)
	}, transferRequestId, &pollResult, client, &transferRequest)

	f.innerResponses <- responseEvent
	if nextState == nil {
		panic("nextState != nil invariant failed")
	}
	return nextState
}

func (f *fsMachine) incrementPollResultIndex(
	transferRequestId string, pollResult *TransferPollResult,
) error {
	if pollResult.Index < pollResult.Total {
		pollResult.Index++
	}
	return updatePollResult(transferRequestId, *pollResult)
}

func (f *fsMachine) retryPush(
	fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
	transferRequestId string, pollResult *TransferPollResult,
	client *JsonRpcClient, transferRequest *TransferRequest,
) (*Event, stateFn) {
	// Let's go!
	var retry int
	var responseEvent *Event
	var nextState stateFn
	for retry < 5 {
		// TODO refactor this wrt retryPull
		responseEvent, nextState = func() (*Event, stateFn) {
			// Interpret empty toSnapshotId as "push to the latest snapshot"
			if toSnapshotId == "" {
				snaps, err := f.state.snapshotsForCurrentMaster(toFilesystemId)
				if err != nil {
					return &Event{
						Name: "failed-getting-local-snapshots", Args: &EventArgs{"err": err},
					}, backoffState
				}
				if len(snaps) == 0 {
					return &Event{
						Name: "no-snapshots-of-that-filesystem",
						Args: &EventArgs{"filesystemId": toFilesystemId},
					}, backoffState
				}
				toSnapshotId = snaps[len(snaps)-1].Id
			}
			log.Printf(
				"[retryPush] from (%s, %s) to (%s, %s), pollResult: %s",
				fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId, pollResult,
			)
			var remoteSnaps []*snapshot
			err := client.CallRemote(
				context.Background(),
				"DatameshRPC.SnapshotsById",
				toFilesystemId,
				&remoteSnaps,
			)
			if err != nil {
				return &Event{
					Name: "failed-getting-remote-snapshots", Args: &EventArgs{"err": err},
				}, backoffState
			}
			fsMachine, err := f.state.maybeFilesystem(toFilesystemId)
			if err != nil {
				return &Event{
					Name: "retry-push-cant-find-filesystem-id",
					Args: &EventArgs{"err": err, "filesystemId": toFilesystemId},
				}, backoffState
			}
			fsMachine.snapshotsLock.Lock()
			snaps := fsMachine.filesystem.snapshots
			fsMachine.snapshotsLock.Unlock()
			// if we're given a target snapshot, restrict f.filesystem.snapshots to
			// that snapshot
			localSnaps, err := restrictSnapshots(snaps, toSnapshotId)
			if err != nil {
				return &Event{
					Name: "restrict-snapshots-error",
					Args: &EventArgs{"err": err, "filesystemId": toFilesystemId},
				}, backoffState
			}
			snapRange, err := canApply(localSnaps, remoteSnaps)
			if err != nil {
				switch err.(type) {
				case *ToSnapsUpToDate:
					// no action, we're up-to-date for this filesystem
					pollResult.Status = "finished"
					pollResult.Message = "remote already up-to-date, nothing to do"

					e := updatePollResult(transferRequestId, *pollResult)
					if e != nil {
						return &Event{
							Name: "push-initiator-cant-write-to-etcd", Args: &EventArgs{"err": e},
						}, backoffState
					}
					return &Event{
						Name: "peer-up-to-date",
					}, backoffState
				}
				return &Event{
					Name: "error-in-canapply-when-pushing", Args: &EventArgs{"err": err},
				}, backoffState
			}
			// TODO peer may error out of pushPeerState, wouldn't we like to get them
			// back into it somehow? we could attempt to do that with by sending a new
			// RegisterTransfer rpc if necessary. or they could retry also.

			var fromSnap string
			if snapRange.fromSnap == nil {
				fromSnap = "START"
				if fromFilesystemId != "" {
					// This is a send from a clone origin
					fromSnap = fmt.Sprintf(
						"%s@%s", fromFilesystemId, fromSnapshotId,
					)
				}
			} else {
				fromSnap = snapRange.fromSnap.Id
			}

			pollResult.FilesystemId = toFilesystemId
			pollResult.StartingSnapshot = fromSnap
			pollResult.TargetSnapshot = snapRange.toSnap.Id

			err = updatePollResult(transferRequestId, *pollResult)
			if err != nil {
				return &Event{
					Name: "push-initiator-cant-write-to-etcd", Args: &EventArgs{"err": err},
				}, backoffState
			}

			// tell the remote what snapshot to expect
			var result bool
			err = client.CallRemote(
				context.Background(), "DatameshRPC.RegisterTransfer", pollResult, &result,
			)
			if err != nil {
				return &Event{
					Name: "push-initiator-cant-register-transfer", Args: &EventArgs{"err": err},
				}, backoffState
			}

			err = updatePollResult(transferRequestId, *pollResult)
			if err != nil {
				return &Event{
					Name: "push-initiator-cant-write-to-etcd", Args: &EventArgs{"err": err},
				}, backoffState
			}

			return f.push(
				fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId,
				snapRange, transferRequest, &transferRequestId, pollResult, client,
			)
		}()
		if responseEvent.Name == "finished-push" || responseEvent.Name == "peer-up-to-date" {
			log.Printf("[actualPush] Successful push!")
			return responseEvent, nextState
		}
		retry++
		f.updateTransfer(
			fmt.Sprintf("retry %d", retry),
			fmt.Sprintf("Attempting to push %s got %s", f.filesystemId, responseEvent),
		)
		log.Printf(
			"[retry attempt %d] squashing and retrying in %ds because we "+
				"got a %s (which tried to put us into %s)...",
			retry, retry, responseEvent, nextState,
		)
		time.Sleep(time.Duration(retry) * time.Second)
	}
	log.Printf(
		"[actualPush] Maximum retry attempts exceeded, "+
			"returning latest error: %s (to move into state %s)",
		responseEvent, nextState,
	)
	return responseEvent, nextState
}

func (f *fsMachine) errorDuringTransfer(desc string, err error) stateFn {
	// for error conditions during a transfer, update innerResponses, and
	// update transfer object, and return a new state
	f.updateTransfer("error", fmt.Sprintf("%s: %s", desc, err))
	f.innerResponses <- &Event{
		Name: desc,
		Args: &EventArgs{"err": err},
	}
	return backoffState
}

func (f *fsMachine) updateTransfer(status, message string) {
	f.lastPollResult.Status = status
	f.lastPollResult.Message = message
	err := updatePollResult(f.lastTransferRequestId, *f.lastPollResult)
	if err != nil {
		// XXX proceeding despite error...
		log.Printf(
			"[updateTransfer] Error while trying to report status: %s => %s",
			message, err,
		)
	}
}

func (f *fsMachine) pull(
	fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
	snapRange *snapshotRange,
	transferRequest *TransferRequest,
	transferRequestId *string,
	pollResult *TransferPollResult,
	client *JsonRpcClient,
) (responseEvent *Event, nextState stateFn) {
	// TODO if we just created the filesystem, become the master for it. (or
	// maybe this belongs in the metadata prenegotiation phase)
	pollResult.Status = "calculating size"
	err := updatePollResult(*transferRequestId, *pollResult)
	if err != nil {
		return &Event{
			Name: "push-initiator-cant-write-to-etcd",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	// TODO dedupe wrt push!!
	// XXX This shouldn't be deduced here _and_ passed in as an argument (which
	// is then thrown away), it just makes the code confusing.
	toFilesystemId = pollResult.FilesystemId
	fromSnapshotId = pollResult.StartingSnapshot

	// 1. Do an RPC to estimate the send size and update pollResult
	// accordingly.
	var size int64
	err = client.CallRemote(context.Background(),
		"DatameshRPC.PredictSize", map[string]interface{}{
			"FromFilesystemId": fromFilesystemId,
			"FromSnapshotId":   fromSnapshotId,
			"ToFilesystemId":   toFilesystemId,
			"ToSnapshotId":     toSnapshotId,
		},
		&size,
	)
	if err != nil {
		return &Event{
			Name: "error-rpc-predict-size",
			Args: &EventArgs{"err": err},
		}, backoffState
	}
	log.Printf("[pull] size: %d", size)
	pollResult.Size = size
	pollResult.Status = "pulling"
	err = updatePollResult(*transferRequestId, *pollResult)
	if err != nil {
		return &Event{
			Name: "push-initiator-cant-write-to-etcd",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	// 2. Perform GET, as receivingState does. Update as we go, similar to how
	// push does it.
	url := fmt.Sprintf(
		"http://%s:6969/filesystems/%s/%s/%s",
		transferRequest.Peer,
		toFilesystemId,
		fromSnapshotId,
		toSnapshotId,
	)
	log.Printf("Pulling from %s", url)
	req, err := http.NewRequest(
		"GET", url, nil,
	)
	req.SetBasicAuth(
		transferRequest.User,
		transferRequest.ApiKey,
	)
	getClient := new(http.Client)
	resp, err := getClient.Do(req)
	if err != nil {
		log.Printf("Attempting to pull %s got %s", fromFilesystemId, err)
		return &Event{
			Name: "get-failed-pull",
			Args: &EventArgs{"err": err, "filesystemId": fromFilesystemId},
		}, backoffState
	}
	log.Printf(
		"Debug: curl -u admin:[pw] http://%s:6969/filesystems/%s/%s/%s",
		transferRequest.Peer, fromFilesystemId, fromSnapshotId, toSnapshotId,
	)
	// TODO finish rewriting return values and update pollResult as the transfer happens...
	cmd := exec.Command("zfs", "recv", fq(f.filesystemId))
	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()
	defer pipeWriter.Close()

	cmd.Stdin = pipeReader
	cmd.Stdout = getLogfile("zfs-recv-stdout")
	cmd.Stderr = getLogfile("zfs-recv-stderr")

	finished := make(chan bool)

	// TODO: make this update the pollResult
	go pipe(
		resp.Body, fmt.Sprintf("http response body for %s", f.filesystemId),
		pipeWriter, "stdin of zfs recv",
		finished,
		f.innerRequests,
		// put the event back on the channel in the cancellation case
		func(e *Event, c chan *Event) { c <- e },
		func(bytes int64, t int64) {
			pollResult.Sent = bytes
			pollResult.NanosecondsElapsed = t
			err = updatePollResult(*transferRequestId, *pollResult)
			if err != nil {
				log.Printf("Error updating poll result: %s", err)
			}
			f.transitionedTo("pull",
				fmt.Sprintf(
					"transferred %.2fMiB in %.2fs (%.2fMiB/s)...",
					// bytes => mebibytes       nanoseconds => seconds
					float64(bytes)/(1024*1024), float64(t)/(1000*1000*1000),
					// mib/sec
					(float64(bytes)/(1024*1024))/(float64(t)/(1000*1000*1000)),
				),
			)
		},
		"decompress",
	)

	log.Printf("[pull] about to start consuming prelude on %v", pipeReader)
	prelude, err := consumePrelude(pipeReader)
	if err != nil {
		return &Event{
			Name: "consume-prelude-failed",
			Args: &EventArgs{"err": err, "filesystemId": f.filesystemId},
		}, backoffState
	}
	log.Printf("[pull] Got prelude %v", prelude)

	err = cmd.Run()
	f.transitionedTo("receiving", "finished zfs recv")
	pipeReader.Close()
	pipeWriter.Close()
	_ = <-finished
	f.transitionedTo("receiving", "finished pipe")

	// XXX why f.filesystemId used and sometimes fromFilesystemId?

	if err != nil {
		log.Printf(
			"Got error %s when running zfs recv for %s, check zfs-recv-stderr.log",
			err, f.filesystemId,
		)
		return &Event{
			Name: "get-failed-pull",
			Args: &EventArgs{"err": err, "filesystemId": fromFilesystemId},
		}, backoffState
	}
	log.Printf("[pull] about to start applying prelude on %v", pipeReader)
	err = applyPrelude(prelude, fq(f.filesystemId))
	if err != nil {
		return &Event{
			Name: "failed-applying-prelude",
			Args: &EventArgs{"err": err, "filesystemId": fromFilesystemId},
		}, backoffState
	}
	pollResult.Status = "finished"
	err = updatePollResult(*transferRequestId, *pollResult)
	if err != nil {
		return &Event{
			Name: "error-updating-poll-result",
			Args: &EventArgs{"err": err},
		}, backoffState
	}
	log.Printf("Successfully received %s => %s for %s", fromSnapshotId, toSnapshotId)
	return &Event{
		Name: "finished-pull",
	}, discoveringState
}

func calculateSendArgs(
	fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
) []string {

	// toFilesystemId
	// snapRange.toSnap.Id
	// snapRange.fromSnap == nil?  --> fromSnapshotId == ""?
	// snapRange.fromSnap.Id

	var sendArgs []string
	var fromSnap string
	if fromSnapshotId == "" {
		fromSnap = "START"
		if fromFilesystemId != "" { // XXX wtf
			// This is a clone-origin based send
			fromSnap = fmt.Sprintf(
				"%s@%s", fromFilesystemId, fromSnapshotId,
			)
		}
	} else {
		fromSnap = fromSnapshotId
	}
	if fromSnap == "START" {
		// -R sends interim snapshots as well
		sendArgs = []string{
			"-p", "-R", fq(toFilesystemId) + "@" + toSnapshotId,
		}
	} else {
		// in clone case, fromSnap must be fully qualified
		if strings.Contains(fromSnap, "@") {
			// send a clone, so make it fully qualified
			fromSnap = fq(fromSnap)
		}
		sendArgs = []string{
			"-p", "-I", fromSnap, fq(toFilesystemId) + "@" + toSnapshotId,
		}
	}
	return sendArgs
}

/*
		Discover total number of bytes in replication stream by asking nicely:

			luke@hostess:/foo$ sudo zfs send -nP pool/foo@now2
			full    pool/foo@now2   105050056
			size    105050056
			luke@hostess:/foo$ sudo zfs send -nP -I pool/foo@now pool/foo@now2
			incremental     now     pool/foo@now2   105044936
			size    105044936

	   -n

		   Do a dry-run ("No-op") send.  Do not generate any actual send
		   data.  This is useful in conjunction with the -v or -P flags to
		   determine what data will be sent.

	   -P

		   Print machine-parsable verbose information about the stream
		   package generated.
*/
func predictSize(
	fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
) (int64, error) {
	sendArgs := calculateSendArgs(fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId)
	predictArgs := []string{"send", "-nP"}
	predictArgs = append(predictArgs, sendArgs...)

	sizeCmd := exec.Command("zfs", predictArgs...)

	log.Printf("[predictSize] predict command: %s", strings.Join(predictArgs, " "))

	out, err := sizeCmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	shrap := strings.Split(string(out), "\n")
	if len(shrap) < 2 {
		return 0, fmt.Errorf("Not enough lines in output %v", string(out))
	}
	sizeLine := shrap[len(shrap)-2]
	shrap = strings.Fields(sizeLine)
	if len(shrap) < 2 {
		return 0, fmt.Errorf("Not enough fields in %v", sizeLine)
	}

	size, err := strconv.ParseInt(shrap[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return size, nil
}

// TODO this method shouldn't really be on a fsMachine, because it is
// parameterized by filesystemId (implicitly in pollResult, which varies over
// phases of a multi-filesystem push)
func (f *fsMachine) push(
	fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
	snapRange *snapshotRange,
	transferRequest *TransferRequest,
	transferRequestId *string,
	pollResult *TransferPollResult,
	client *JsonRpcClient,
) (responseEvent *Event, nextState stateFn) {

	filesystemId := pollResult.FilesystemId

	// XXX This shouldn't be deduced here _and_ passed in as an argument (which
	// is then thrown away), it just makes the code confusing.
	fromSnapshotId = pollResult.StartingSnapshot

	pollResult.Status = "calculating size"
	err := updatePollResult(*transferRequestId, *pollResult)
	if err != nil {
		return &Event{
			Name: "push-initiator-cant-write-to-etcd",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	postReader, postWriter := io.Pipe()

	defer postWriter.Close()
	defer postReader.Close()

	url := fmt.Sprintf(
		"http://%s:6969/filesystems/%s/%s/%s",
		transferRequest.Peer,
		filesystemId,
		fromSnapshotId,
		snapRange.toSnap.Id,
	)
	log.Printf("Pushing to %s", url)
	req, err := http.NewRequest(
		"POST", url,
		postReader,
	)
	if err != nil {
		log.Printf("Attempting to push %s got %s", filesystemId, err)
		return &Event{
			Name: "error-starting-post-when-pushing",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	// TODO remove duplication (with replication.go)
	// command writes into pipe
	var cmd *exec.Cmd
	// https://github.com/lukemarsden/datamesh/issues/34
	// https://github.com/zfsonlinux/zfs/pull/5189
	//
	// Due to the above issues, -R doesn't send user properties on
	// platforms we care about (notably, the version of ZFS that is bundled
	// with Ubuntu 16.04 and 16.10).
	//
	// Workaround this limitation by include the missing information in
	// JSON format in a "prelude" section of the ZFS send stream.
	//
	prelude, err := f.state.calculatePrelude(toFilesystemId, toSnapshotId)
	if err != nil {
		return &Event{
			Name: "error-calculating-prelude",
			Args: &EventArgs{"err": err, "filesystemId": toFilesystemId},
		}, backoffState
	}

	// TODO test whether toFilesystemId and toSnapshotId are set correctly,
	// and consistently with snapRange?
	sendArgs := calculateSendArgs(
		fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId,
	)
	realArgs := []string{"send"}
	realArgs = append(realArgs, sendArgs...)

	// XXX this doesn't need to happen every push(), just once above.
	size, err := predictSize(
		fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId,
	)
	if err != nil {
		return &Event{
			Name: "error-predicting",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	log.Printf("[actualPush] size: %d", size)
	pollResult.Size = size
	pollResult.Status = "pushing"
	err = updatePollResult(*transferRequestId, *pollResult)
	if err != nil {
		return &Event{
			Name: "push-initiator-cant-write-to-etcd",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	// proceed to do real send
	cmd = exec.Command("zfs", realArgs...)
	pipeReader, pipeWriter := io.Pipe()

	defer pipeWriter.Close()
	defer pipeReader.Close()

	// we will write this to the pipe first, in the goroutine which writes
	preludeEncoded, err := encodePrelude(prelude)
	if err != nil {
		return &Event{
			Name: "cant-encode-prelude",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	cmd.Stdout = pipeWriter
	cmd.Stderr = getLogfile("zfs-send-errors")

	finished := make(chan bool)
	go pipe(
		pipeReader, fmt.Sprintf("stdout of zfs send for %s", filesystemId),
		postWriter, "http request body",
		finished,
		make(chan *Event),
		func(e *Event, c chan *Event) {},
		func(bytes int64, t int64) {
			pollResult.Sent = bytes
			pollResult.NanosecondsElapsed = t
			err = updatePollResult(*transferRequestId, *pollResult)
			if err != nil {
				log.Printf("Error updating poll result: %s", err)
			}
			f.transitionedTo("pushInitiatorState",
				fmt.Sprintf(
					"transferred %.2fMiB in %.2fs (%.2fMiB/s)...",
					// bytes => mebibytes       nanoseconds => seconds
					float64(bytes)/(1024*1024), float64(t)/(1000*1000*1000),
					// mib/sec
					(float64(bytes)/(1024*1024))/(float64(t)/(1000*1000*1000)),
				),
			)
		},
		"compress",
	)

	log.Printf(
		"[actualPush] Writing prelude of %d bytes (encoded): %s",
		len(preludeEncoded), preludeEncoded,
	)
	pipeWriter.Write(preludeEncoded)

	req.SetBasicAuth(
		transferRequest.User,
		transferRequest.ApiKey,
	)
	postClient := new(http.Client)

	log.Printf("About to postClient.Do with req %s", req)

	// postClient.Do will block trying to read the first byte of the request
	// body. But, we won't be able to provide the first byte until we start
	// running the command. So, do what we always do to avoid a deadlock. Run
	// something in a goroutine. In this case we need 'resp' in scope, so let's
	// run the command in a goroutine.

	errch := make(chan error)
	go func() {
		log.Printf(
			"[actualPush] About to Run() for %s %s => %s",
			filesystemId, fromSnapshotId, toSnapshotId,
		)
		runErr := cmd.Run()

		log.Printf(
			"[actualPush] Run() got result %s, about to put it into errch after closing pipeWriter",
			runErr,
		)
		err := pipeWriter.Close()
		if err != nil {
			log.Printf("[actualPush] error closing pipeWriter: %s", err)
		}
		errch <- runErr
		log.Printf("[actualPush] errch accepted it, woohoo")
	}()

	resp, err := postClient.Do(req)
	if err != nil {
		log.Printf("[actualPush] error in postClient.Do: %s", err)
		return &Event{
			Name: "error-from-post-when-pushing",
			Args: &EventArgs{"err": err},
		}, backoffState
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf(
			"[actualPush] Got error while reading response body %s: %s",
			string(responseBody), err,
		)
		return &Event{
			Name: "error-reading-push-response-body",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	if resp.StatusCode != 200 {
		return &Event{
			Name: "error-pushing-posting",
			Args: &EventArgs{
				"requestURL":      url,
				"responseBody":    string(responseBody),
				"statusCode":      fmt.Sprintf("%d", resp.StatusCode),
				"responseHeaders": fmt.Sprintf("%+v", resp.Header),
			},
		}, backoffState
	}

	log.Printf("[actualPush] Got response body while pushing: %s", string(responseBody))

	log.Printf("[actualPush] Waiting for finish signal...")
	_ = <-finished
	log.Printf("[actualPush] Done!")

	log.Printf("[actualPush] reading from errch")
	err = <-errch
	log.Printf(
		"[actualPush] Finished Run() for %s %s => %s: %s",
		filesystemId, fromSnapshotId, toSnapshotId, err,
	)
	if err != nil {
		log.Printf(
			"[actualPush] Error from zfs send of %s from %s => %s: %s, check zfs-send-errors.log",
			filesystemId, fromSnapshotId, toSnapshotId, err,
		)
		return &Event{
			Name: "error-from-zfs-send",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	// XXX Adding the log messages below seemed to stop a deadlock, not sure
	// why. For now, let's just leave them in...
	// XXX what about closing post{Writer,Reader}?
	log.Printf("[actualPush] Closing pipes...")
	pipeWriter.Close()
	pipeReader.Close()

	pollResult.Status = "finished"
	err = updatePollResult(*transferRequestId, *pollResult)
	if err != nil {
		return &Event{
			Name: "error-updating-poll-result",
			Args: &EventArgs{"err": err},
		}, backoffState
	}

	// TODO update the transfer record, release the peer state machines
	return &Event{
		Name: "finished-push",
		Args: &EventArgs{},
	}, discoveringState
}

func pushPeerState(f *fsMachine) stateFn {
	// we are responsible for putting something back onto the channel
	f.transitionedTo("pushPeerState", "running")

	newSnapsOnMaster := make(chan interface{})
	receiveProgress := make(chan interface{})
	log.Printf("[pushPeerState] subscribing to newSnapsOnMaster for %s", f.filesystemId)

	f.state.localReceiveProgress.Subscribe(f.filesystemId, receiveProgress)
	defer f.state.localReceiveProgress.Unsubscribe(f.filesystemId, receiveProgress)

	f.state.newSnapsOnMaster.Subscribe(f.filesystemId, newSnapsOnMaster)
	defer f.state.newSnapsOnMaster.Unsubscribe(f.filesystemId, newSnapsOnMaster)

	// this is a write state. refuse to act if containers are running

	// refuse to be pushed into if we have any containers running
	// TODO stop any containers being started, somehow.
	containers, err := f.containersRunning()
	if err != nil {
		log.Printf(
			"Can't receive push for filesystem while we can't list whether containers are using it",
		)
		f.innerResponses <- &Event{
			Name: "error-listing-containers-during-push-receive",
			Args: &EventArgs{"err": err},
		}
		return backoffState
	}
	if len(containers) > 0 {
		log.Printf("Can't receive push for filesystem while containers are using it")
		f.innerResponses <- &Event{
			Name: "cannot-receive-push-while-containers-running",
			Args: &EventArgs{"containers": containers},
		}
		return backoffState
	}

	// wait for the desired snapshot to exist here. this means that completing
	// a receive operation must prompt us into loading, but without forgetting
	// that we were in here, so some kind of inline-loading.

	// what is the desired snapshot?
	targetSnapshot := f.lastTransferRequest.TargetSnapshot

	// XXX are we allowed to transitively receive into other filesystems,
	// without synchronizing with their state machines?

	// first check whether we already have the snapshot. if so, early
	// exit?
	ss, err := f.state.snapshotsFor(f.state.myNodeId, f.filesystemId)
	for _, s := range ss {
		if s.Id == targetSnapshot {
			f.innerResponses <- &Event{
				Name: "receiving-push-complete",
				Args: &EventArgs{},
			}
			log.Printf(
				"[pushPeerState:%s] snaps-already-exist case, "+
					"returning activeState on snap %s",
				f.filesystemId, targetSnapshot,
			)
			return activeState
		}
	}

	timeoutTimer := time.NewTimer(600 * time.Second)
	finished := make(chan bool)

	// reset timer when progress is made
	reset := func() {
		// copied from https://golang.org/pkg/time/#Timer.Reset
		if !timeoutTimer.Stop() {
			<-timeoutTimer.C
		}
		timeoutTimer.Reset(600 * time.Second)
	}

	go func() {
		for {
			select {
			case <-receiveProgress:
				//log.Printf(
				//	"[pushPeerState] resetting timer because some progress was made (%d bytes)", b,
				//)
				reset()
			case <-finished:
				return
			}
		}
	}()

	// allow timer-resetter goroutine to exit as soon as we exit this function
	defer func() {
		go func() {
			finished <- true
		}()
	}()

	for {
		log.Printf("[pushPeerState:%s] about to read from externalSnapshotsChanged", f.filesystemId)

		select {
		case <-timeoutTimer.C:
			log.Printf(
				"[pushPeerState:%s] Timed out waiting for externalSnapshotsChanged",
				f.filesystemId,
			)
			f.innerResponses <- &Event{
				Name: "timed-out-external-snaps",
				Args: &EventArgs{},
			}
			return backoffState
		case <-f.externalSnapshotsChanged:
			// onwards!
		}
		log.Printf(
			"[pushPeerState:%s] read from externalSnapshotsChanged! doing inline load",
			f.filesystemId,
		)

		// inline load, async because discover() blocks on publishing to
		// newSnapsOnMaster chan, which we're subscribed to and so have to read
		// from concurrently with discover() to avoid deadlock.
		go func() {
			err = f.discover()
			log.Printf("[pushPeerState] done inline load")
			if err != nil {
				// XXX how to propogate the error to the initiator? should their
				// retry include sending a new peer-transfer message every time?
				log.Printf("[pushPeerState] error during inline load: %s", err)
			}
		}()

		// give ourselves another 60 seconds while loading
		log.Printf("[pushPeerState] resetting timer because we're waiting for loading")
		reset()

		log.Printf("[pushPeerState] about to read from newSnapsOnMaster")
		select {
		case <-timeoutTimer.C:
			log.Printf("[pushPeerState] Timed out waiting for newSnapsOnMaster")
			f.innerResponses <- &Event{
				Name: "timed-out-snaps-on-master",
				Args: &EventArgs{},
			}
			return backoffState
		// check that the snapshot is the one we're expecting
		case s := <-newSnapsOnMaster:
			sn := s.(snapshot)
			log.Printf(
				"[pushPeerState] got snapshot %s while waiting for one to arrive", sn,
			)
			if sn.Id == targetSnapshot {
				log.Printf(
					"[pushPeerState] %s matches target snapshot %s!",
					sn.Id, targetSnapshot,
				)
				f.snapshotsLock.Lock()
				mounted := f.filesystem.mounted
				f.snapshotsLock.Unlock()
				if mounted {
					f.innerResponses <- &Event{
						Name: "receiving-push-complete",
						Args: &EventArgs{},
					}
					log.Printf(
						"[pushPeerState:%s] mounted case, returning activeState on snap %s",
						f.filesystemId, sn.Id,
					)
					return activeState
				} else {
					// XXX does mounting alone dirty the filesystem, stopping
					// receiving further pushes?
					responseEvent, nextState := f.mount()
					if responseEvent.Name == "mounted" {
						f.innerResponses <- &Event{
							Name: "receiving-push-complete",
							Args: &EventArgs{},
						}
					} else {
						f.innerResponses <- responseEvent
					}
					log.Printf(
						"[pushPeerState:%s] unmounted case, returning nextState %s on snap %s",
						f.filesystemId, nextState, sn.Id,
					)
					return nextState
				}
			} else {
				log.Printf(
					"[pushPeerState] %s doesn't match target snapshot %s, "+
						"waiting for another...", sn.Id, targetSnapshot,
				)
			}
		}
	}
}

func pullInitiatorState(f *fsMachine) stateFn {
	f.transitionedTo("pullInitiatorState", "requesting")
	// this is a write state. refuse to act if containers are running

	// refuse to pull if we have any containers running
	// TODO stop any containers being started, somehow. (by acquiring a lock?)
	containers, err := f.containersRunning()
	if err != nil {
		log.Printf(
			"Can't pull into filesystem while we can't list whether containers are using it",
		)
		f.innerResponses <- &Event{
			Name: "error-listing-containers-during-pull",
			Args: &EventArgs{"err": err},
		}
		return backoffState
	}
	if len(containers) > 0 {
		log.Printf("Can't pull into filesystem while containers are using it")
		f.innerResponses <- &Event{
			Name: "cannot-pull-while-containers-running",
			Args: &EventArgs{"containers": containers},
		}
		return backoffState
	}

	transferRequest := f.lastTransferRequest
	transferRequestId := f.lastTransferRequestId

	// TODO dedupe what follows wrt pushInitiatorState!
	client := NewJsonRpcClient(
		transferRequest.User,
		transferRequest.Peer,
		transferRequest.ApiKey,
	)

	var path PathToTopLevelFilesystem
	// XXX Not propagating context here; not needed for auth, but would be nice
	// for inter-cluster opentracing.
	err = client.CallRemote(context.Background(),
		"DatameshRPC.DeducePathToTopLevelFilesystem", map[string]interface{}{
			"RemoteFilesystemName": transferRequest.RemoteFilesystemName,
			"RemoteCloneName":      transferRequest.RemoteCloneName,
		},
		&path,
	)
	if err != nil {
		f.innerResponses <- &Event{
			Name: "cant-rpc-deduce-path",
			Args: &EventArgs{"err": err},
		}
		return backoffState
	}

	// register a poll result object.
	pollResult := TransferPollResultFromTransferRequest(
		transferRequestId, transferRequest, f.state.myNodeId,
		1, 1+len(path.Clones), "syncing metadata",
	)
	f.lastPollResult = &pollResult

	err = updatePollResult(transferRequestId, pollResult)
	if err != nil {
		f.innerResponses <- &Event{
			Name: "pull-initiator-cant-write-to-etcd",
			Args: &EventArgs{"err": err},
		}
		return backoffState
	}

	// iterate over the path, attempting to pull each clone in turn.
	responseEvent, nextState := f.applyPath(path, func(f *fsMachine,
		fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
		transferRequestId string, pollResult *TransferPollResult,
		client *JsonRpcClient, transferRequest *TransferRequest,
	) (*Event, stateFn) {
		return f.retryPull(
			fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId,
			transferRequestId, pollResult, client, transferRequest,
		)
	}, transferRequestId, &pollResult, client, &transferRequest)

	f.innerResponses <- responseEvent
	return nextState
}

func (f *fsMachine) retryPull(
	fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
	transferRequestId string, pollResult *TransferPollResult,
	client *JsonRpcClient, transferRequest *TransferRequest,
) (*Event, stateFn) {
	// TODO refactor the following with respect to retryPush!

	// Let's go!
	var remoteSnaps []*snapshot
	err := client.CallRemote(
		context.Background(),
		"DatameshRPC.SnapshotsById",
		toFilesystemId,
		&remoteSnaps,
	)
	if err != nil {
		return &Event{
			Name: "failed-getting-snapshots", Args: &EventArgs{"err": err},
		}, backoffState
	}

	// Interpret empty toSnapshotId as "pull up to the latest snapshot" _on the
	// remote_
	if toSnapshotId == "" {
		if len(remoteSnaps) == 0 {
			return &Event{
				Name: "no-snapshots-of-remote-filesystem",
				Args: &EventArgs{"filesystemId": toFilesystemId},
			}, backoffState
		}
		toSnapshotId = remoteSnaps[len(remoteSnaps)-1].Id
	}
	log.Printf(
		"[retryPull] from (%s, %s) to (%s, %s), pollResult: %s",
		fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId, pollResult,
	)

	fsMachine, err := f.state.maybeFilesystem(toFilesystemId)
	if err != nil {
		return &Event{
			Name: "retry-pull-cant-find-filesystem-id",
			Args: &EventArgs{"err": err, "filesystemId": toFilesystemId},
		}, backoffState
	}
	fsMachine.snapshotsLock.Lock()
	localSnaps := fsMachine.filesystem.snapshots
	fsMachine.snapshotsLock.Unlock()
	// if we're given a target snapshot, restrict f.filesystem.snapshots to
	// that snapshot
	remoteSnaps, err = restrictSnapshots(remoteSnaps, toSnapshotId)
	if err != nil {
		return &Event{
			Name: "restrict-snapshots-error",
			Args: &EventArgs{"err": err, "filesystemId": toFilesystemId},
		}, backoffState
	}
	snapRange, err := canApply(remoteSnaps, localSnaps)
	if err != nil {
		switch err.(type) {
		case *ToSnapsUpToDate:
			// no action, we're up-to-date for this filesystem
			pollResult.Status = "finished"
			pollResult.Message = "remote already up-to-date, nothing to do"

			e := updatePollResult(transferRequestId, *pollResult)
			if e != nil {
				return &Event{
					Name: "pull-initiator-cant-write-to-etcd", Args: &EventArgs{"err": e},
				}, backoffState
			}
			return &Event{
				Name: "peer-up-to-date",
			}, backoffState
		}
		return &Event{
			Name: "error-in-canapply-when-pulling", Args: &EventArgs{"err": err},
		}, backoffState
	}
	var fromSnap string
	// XXX dedupe this wrt calculateSendArgs/predictSize
	if snapRange.fromSnap == nil {
		fromSnap = "START"
		if fromFilesystemId != "" {
			// This is a receive from a clone origin
			fromSnap = fmt.Sprintf(
				"%s@%s", fromFilesystemId, fromSnapshotId,
			)
		}
	} else {
		fromSnap = snapRange.fromSnap.Id
	}

	pollResult.FilesystemId = toFilesystemId
	pollResult.StartingSnapshot = fromSnap
	pollResult.TargetSnapshot = snapRange.toSnap.Id

	err = updatePollResult(transferRequestId, *pollResult)
	if err != nil {
		return &Event{
			Name: "pull-initiator-cant-write-to-etcd", Args: &EventArgs{"err": err},
		}, backoffState
	}

	err = updatePollResult(transferRequestId, *pollResult)
	if err != nil {
		return &Event{
			Name: "pull-initiator-cant-write-to-etcd", Args: &EventArgs{"err": err},
		}, backoffState
	}

	var retry int
	var responseEvent *Event
	var nextState stateFn
	for retry < 5 {
		// XXX XXX XXX REFACTOR (retryPush)
		responseEvent, nextState = f.pull(
			fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId,
			snapRange, transferRequest, &transferRequestId, pollResult, client,
		)
		if responseEvent.Name == "finished-pull" || responseEvent.Name == "peer-up-to-date" {
			log.Printf("[actualPull] Successful pull!")
			return responseEvent, nextState
		}
		retry++
		f.updateTransfer(
			fmt.Sprintf("retry %d", retry),
			fmt.Sprintf("Attempting to pull %s got %s", f.filesystemId, responseEvent),
		)
		log.Printf(
			"[retry attempt %d] squashing and retrying in %ds because we "+
				"got a %s (which tried to put us into %s)...",
			retry, retry, responseEvent, nextState,
		)
		time.Sleep(time.Duration(retry) * time.Second)
	}
	log.Printf(
		"[actualPull] Maximum retry attempts exceeded, "+
			"returning latest error: %s (to move into state %s)",
		responseEvent, nextState,
	)
	return &Event{
		Name: "maximum-retry-attempts-exceeded", Args: &EventArgs{"responseEvent": responseEvent},
	}, backoffState
}

func pullPeerState(f *fsMachine) stateFn {
	// This is kind-of a boring state. An authenticated user can GET a
	// filesystem whenever. So arguably a valid implementation of pullPeerState
	// is just to immediately go back to discoveringState. In the future, it
	// might be nicer for observability to synchronize staying in this state
	// until our peer has what it needs. And maybe we want to block some other
	// events while this is happening? (Although I think we want to do that for
	// GETs in general?)
	f.transitionedTo("pullPeerState", "immediate-return")
	return discoveringState
}

// for each clone, ensure its origin snapshot exists on the remote. if it
// doesn't, transfer it.
func (f *fsMachine) applyPath(
	path PathToTopLevelFilesystem, transferFn transferFn,
	transferRequestId string, pollResult *TransferPollResult,
	client *JsonRpcClient, transferRequest *TransferRequest,
) (*Event, stateFn) {
	/*
		Case 1: single master filesystem
		--------------------------------

		TopLevelFilesystemId: <master branch filesystem id>
		TopLevelFilesystemName: foo
		Clones: []

		transferFn("", "", "<master branch filesystem id>", "")

		Case 2: branch-of-branch-of-master (for example)
		------------------------------------------------

		TopLevelFilesystemId: <master branch filesystem id>
		TopLevelFilesystemName: foo
		Clones: []Clone{
			Clone{
				FilesystemId: <branch1 filesystem id>,
				Origin: {
					FilesystemId: <master branch filesystem id>,
					SnapshotId: <snapshot that is origin on master branch>,
				}
			},
			Clone{
				FilesystemId: <branch2 filesystem id>,
				Origin: {
					FilesystemId: <branch1 filesystem id>,
					SnapshotId: <snapshot that is origin on branch1 branch>,
				}
			},
		}

		Required actions:

		push master branch from:
			beginning to:
				snapshot that is origin on master branch
		push branch1 from:
			snapshot that is origin on master branch, to:
				snapshot that is origin on branch1 branch
		push branch2 from:
			snapshot that is origin on branch1 branch, to:
				latest snapshot on branch2

		Examples:

		transferFn("", "", "<master branch filesystem id>", "<origin snapshot on master>")

		push master branch from:
			beginning to:
				snapshot that is origin on master branch

		transferFn(
			"<master branch filesystem id>", "<origin snapshot on master>",
			"<branch1 filesystem id>", "<origin snapshot on branch1>",
		)

		push branch1 from:
			snapshot that is origin on master branch, to:
				snapshot that is origin on branch1 branch

		transferFn(
			"<branch1 branch filesystem id>", "<origin snapshot on branch1>",
			"<branch2 filesystem id>", "",
		)

		push branch2 from:
			snapshot that is origin on branch1 branch, to:
				latest snapshot on branch2
	*/

	var responseEvent *Event
	var nextState stateFn
	var firstSnapshot string
	if len(path.Clones) == 0 {
		// just pushing a master branch to its latest snapshot
		// do a push with empty origin and empty target snapshot
		// TODO parametrize "push to snapshot" and expose in the UI
		firstSnapshot = ""
	} else {
		// push the master branch up to the first snapshot
		firstSnapshot = path.Clones[0].Clone.Origin.SnapshotId
	}
	log.Printf(
		"[applyPath,b] calling transferFn with fF=%v, fS=%v, tF=%v, tS=%v",
		"", "", path.TopLevelFilesystemId, firstSnapshot,
	)
	responseEvent, nextState = transferFn(f,
		"", "", path.TopLevelFilesystemId, firstSnapshot,
		transferRequestId, pollResult, client, transferRequest,
	)
	if !(responseEvent.Name == "finished-push" ||
		responseEvent.Name == "finished-pull" || responseEvent.Name == "peer-up-to-date") {
		msg := fmt.Sprintf(
			"Response event != finished-{push,pull} or peer-up-to-date: %s", responseEvent,
		)
		f.updateTransfer("error", msg)
		return &Event{
			Name: "error-in-attempting-apply-path",
			Args: &EventArgs{
				"error": msg,
			},
		}, backoffState
	}
	err := f.state.maybeMountFilesystem(path.TopLevelFilesystemId)
	if err != nil {
		return &Event{
			Name: "error-maybe-mounting-filesystem",
			Args: &EventArgs{"error": err, "filesystemId": path.TopLevelFilesystemId},
		}, backoffState
	}
	err = f.incrementPollResultIndex(transferRequestId, pollResult)
	if err != nil {
		return &Event{Name: "error-incrementing-poll-result",
			Args: &EventArgs{"error": err}}, backoffState
	}

	for i, clone := range path.Clones {
		// default empty-strings is fine
		nextOrigin := Origin{}
		// is there a next (i+1'th) item? (i is zero-indexed)
		if len(path.Clones) > i+1 {
			// example: path.Clones is 2 items long, and we're on the second
			// one; i=1, len(path.Clones) = 2; 2 > 2 is false; so we're on the
			// last item so the guard evaluates to false; if we're on the first
			// item, 2 > 1 is true, so guard is true.
			nextOrigin = path.Clones[i+1].Clone.Origin
		}
		log.Printf(
			"[applyPath,i] calling transferFn with fF=%v, fS=%v, tF=%v, tS=%v",
			clone.Clone.Origin.FilesystemId, clone.Clone.Origin.SnapshotId,
			clone.Clone.FilesystemId, nextOrigin.SnapshotId,
		)
		responseEvent, nextState = transferFn(f,
			clone.Clone.Origin.FilesystemId, clone.Clone.Origin.SnapshotId,
			clone.Clone.FilesystemId, nextOrigin.SnapshotId,
			transferRequestId, pollResult, client, transferRequest,
		)
		if !(responseEvent.Name == "finished-push" ||
			responseEvent.Name == "finished-pull" || responseEvent.Name == "peer-up-to-date") {
			msg := fmt.Sprintf(
				"Response event != finished-{push,pull} or peer-up-to-date: %s", responseEvent,
			)
			f.updateTransfer("error", msg)
			return &Event{
					Name: "error-in-attempting-apply-path",
					Args: &EventArgs{
						"error": msg,
					},
				},
				backoffState
		}
		err := f.state.maybeMountFilesystem(clone.Clone.FilesystemId)
		if err != nil {
			return &Event{
				Name: "error-maybe-mounting-filesystem",
				Args: &EventArgs{"error": err, "filesystemId": clone.Clone.FilesystemId},
			}, backoffState
		}
		err = f.incrementPollResultIndex(transferRequestId, pollResult)
		if err != nil {
			return &Event{Name: "error-incrementing-poll-result"},
				backoffState
		}
	}
	return responseEvent, nextState
}

// TODO: spin up _three_ single node clusters, use one as a hub so that alice
// and bob can collaborate.

// TODO: run dind/dind-cluster.sh up, and then test the manifests in
// kubernetes/ against the resulting (3 node by default) cluster. Ensure things
// run offline. Figure out how to configure each cluster node with its own
// zpool. Test dynamic provisioning, and so on.
