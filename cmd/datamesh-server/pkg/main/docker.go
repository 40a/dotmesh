package main

// docker volume plugin for providing datamesh volumes to docker via e.g.
// docker run -v name:/path --volume-driver=dm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const PLUGINS_DIR = "/run/docker/plugins"
const DM_SOCKET = PLUGINS_DIR + "/dm.sock"

type ResponseImplements struct {
	// A response to the Plugin.Activate request
	Implements []string
}

type RequestCreate struct {
	// A request to create a volume for Docker
	Name string
	Opts map[string]string
}

type RequestMount struct {
	// A request to mount a volume for Docker
	Name string
}

type RequestGet struct {
	// A request to get a volume for Docker
	Name string
}

type RequestRemove struct {
	// A request to remove a volume for Docker
	Name string
}

type ResponseSimple struct {
	// A response which only indicates if there was an error or not
	Err string
}

type ResponseMount struct {
	// A response to the VolumeDriver.Mount request
	Mountpoint string
	Err        string
}

type ResponseListVolume struct {
	// Used in the JSON representation of ResponseList
	Name       string
	Mountpoint string
	Status     map[string]string // TODO actually start using the status to report on things in dm
}

type ResponseList struct {
	// A response which enumerates volumes for VolumeDriver.List
	Volumes []ResponseListVolume
	Err     string
}

type ResponseGet struct {
	// A response which enumerates volumes for VolumeDriver.Get
	Volume ResponseListVolume
	Err    string
}

// create a symlink from /datamesh/:name[@:branch] into /dmfs/:filesystemId
func newContainerMountSymlink(name VolumeName, filesystemId string) (string, error) {
	if _, err := os.Stat(CONTAINER_MOUNT_PREFIX); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(CONTAINER_MOUNT_PREFIX, 0700); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	if _, err := os.Stat(containerMntParent(name)); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(containerMntParent(name), 0700); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	result := containerMnt(name)
	// Only create it if it doesn't already exist. Otherwise just hand it back
	// (the target of it may have been updated elsewhere).
	if _, err := os.Stat(result); err != nil {
		if os.IsNotExist(err) {
			err = os.Symlink(mnt(filesystemId), result)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	return result, nil
}

func (state *InMemoryState) mustCleanupSocket() {
	if _, err := os.Stat(PLUGINS_DIR); err != nil {
		if err := os.MkdirAll(PLUGINS_DIR, 0700); err != nil {
			log.Fatalf("Could not make plugin directory %s: %v", PLUGINS_DIR, err)
		}
	}
	if _, err := os.Stat(DM_SOCKET); err == nil {
		if err = os.Remove(DM_SOCKET); err != nil {
			log.Fatalf("Could not clean up existing socket at %s: %v", DM_SOCKET, err)
		}
	}
}

// Annotate a context with admin-level authorization.
func AdminContext(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, "authenticated-user-id", ADMIN_USER_UUID)
	return ctx
}

func (state *InMemoryState) runPlugin() {
	log.Print("Starting dm plugin")

	// docker acts like the admin user, for now.
	ctx := AdminContext(context.Background())

	state.mustCleanupSocket()

	http.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		log.Print("<= /Plugin.Activate")
		responseJSON, _ := json.Marshal(&ResponseImplements{
			Implements: []string{"VolumeDriver"},
		})
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
	})
	reallyProcureFilesystem := func(name VolumeName) (string, error) {
		// move filesystem here if it's not here already (coordinate the move
		// with the current master via etcd), also (TODO check this) DON'T
		// ALLOW PATH TO BE PASSED TO DOCKER IF IT IS NOT ACTUALLY MOUNTED
		// (otherwise databases will show up as empty)

		// If the filesystem exists anywhere in the cluster, and a small amount
		// of time has passed, we should have an inactive filesystem state
		// machine.

		cloneName := ""
		if strings.Contains(name.Name, "@") {
			shrapnel := strings.Split(name.Name, "@")
			name.Name = shrapnel[0]
			cloneName = shrapnel[1]
			if cloneName == DEFAULT_BRANCH {
				cloneName = ""
			}
		}

		log.Printf(
			"*** Attempting to procure filesystem name %s and clone name %s",
			name, cloneName,
		)

		filesystemId, err := state.registry.MaybeCloneFilesystemId(name, cloneName)
		if err == nil {
			// TODO can we synchronize with the state machine somehow, to
			// ensure that we're not currently on a master in the process of
			// doing a handoff?
			if state.masterFor(filesystemId) == state.myNodeId {
				log.Printf("Volume already here, we are done %s", filesystemId)
				return filesystemId, nil
			} else {
				// put in a request for the current master of the filesystem to
				// move it to me
				responseChan, err := state.globalFsRequest(
					filesystemId,
					&Event{
						Name: "move",
						Args: &EventArgs{"target": state.myNodeId},
					},
				)
				if err != nil {
					return "", err
				}
				log.Printf(
					"Attempting to move %s from %s to me (%s)",
					filesystemId,
					state.masterFor(filesystemId),
					state.myNodeId,
				)
				var e *Event
				select {
				case <-time.After(30 * time.Second):
					// something needs to read the response from the
					// response chan
					go func() { _ = <-responseChan }()
					// TODO implement some kind of liveness check to avoid
					// timing out too early on slow transfers.
					return "", fmt.Errorf(
						"timed out trying to procure %s, please try again", filesystemId,
					)
				case e = <-responseChan:
					// tally ho!
				}
				log.Printf(
					"Attempting to move %s from %s to me (%s)",
					filesystemId, state.masterFor(filesystemId), state.myNodeId,
				)
				if e.Name != "moved" {
					return "", fmt.Errorf(
						"failed to move %s from %s to %s: %s",
						filesystemId, state.masterFor(filesystemId), state.myNodeId, e,
					)
				}
				// great - the current master thinks it's handed off to us.
				// doesn't mean we've actually mounted the filesystem yet
				// though, so wait on that here.

				state.filesystemsLock.Lock()
				if (*state.filesystems)[filesystemId].currentState == "active" {
					// great - we're already active
					log.Printf("Found %s was already active, giving it to Docker", filesystemId)
					state.filesystemsLock.Unlock()
				} else {
					for (*state.filesystems)[filesystemId].currentState != "active" {
						log.Printf(
							"%s was %s, waiting for it to change to active...",
							filesystemId, (*state.filesystems)[filesystemId].currentState,
						)
						// wait for state change
						stateChangeChan := make(chan interface{})
						(*state.filesystems)[filesystemId].transitionObserver.Subscribe(
							"transitions", stateChangeChan,
						)
						state.filesystemsLock.Unlock()
						_ = <-stateChangeChan
						state.filesystemsLock.Lock()
						(*state.filesystems)[filesystemId].transitionObserver.Unsubscribe(
							"transitions", stateChangeChan,
						)
					}
					log.Printf("%s finally changed to active, proceeding!", filesystemId)
					state.filesystemsLock.Unlock()
				}
			}
		} else {
			fsMachine, ch, err := state.CreateFilesystem(ctx, &name)
			if err != nil {
				return "", err
			}
			filesystemId = fsMachine.filesystemId
			if cloneName != "" {
				return "", fmt.Errorf("Cannot use branch-pinning syntax (docker run -v volume@branch:/path) to create a non-existent volume with a non-master branch")
			}
			log.Printf("WAITING FOR CREATE %s", name)
			e := <-ch
			if e.Name != "created" {
				return "", fmt.Errorf("Could not create volume %s: unexpected response %s - %s", name, e.Name, e.Args)
			}
			log.Printf("DONE CREATE %s", name)
		}
		return filesystemId, nil
	}
	procureFilesystem := func(name VolumeName) (string, error) {
		s, err := reallyProcureFilesystem(name)
		if err != nil {
			// retry once, to handle the case where we race with another node
			// to claim a name, and etcd protects us; it's possible we want to
			// move the filesystem instead. delay is needed because we're
			// waiting with a watch to fire with our updated knowledge, if we
			// retry immediately, we're likely to just consult our stale cache
			// again.
			log.Printf(
				"[procureFilesystem] Retrying reallyProcureFilesystem(%s) because of %s in 5s",
				name, err,
			)
			time.Sleep(5 * time.Second)
			log.Printf("[procureFilesystem] Retrying reallyProcureFilesystem(%s) now", name, err)
			return reallyProcureFilesystem(name)
		}
		return s, err
	}

	http.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		log.Print("<= /VolumeDriver.Create")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		request := new(RequestCreate)
		err = json.Unmarshal(requestJSON, request)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		// for now, just name the volumes as requested by the user. later,
		// adding ids and per-fs metadata may be useful.

		if _, err := procureFilesystem(name); err != nil {
			writeResponseErr(err, w)
			return
		}
		// TODO acquire containerRuntimeLock and update our state and etcd with
		// the fact that a container will soon be running on this volume...
		writeResponseOK(w)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	http.HandleFunc("/VolumeDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		/*
			We do not actually want to remove the dm volume when Docker
			references to them are removed.

			This is a no-op.
		*/
		writeResponseOK(w)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	http.HandleFunc("/VolumeDriver.Path", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Only return the path if it's actually active on the local host.
		log.Print("<= /VolumeDriver.Path")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		request := new(RequestMount)
		if err := json.Unmarshal(requestJSON, request); err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		log.Printf("Mountpoint for %s: %s", name, containerMnt(name))
		responseJSON, _ := json.Marshal(&ResponseMount{
			Mountpoint: containerMnt(name),
			Err:        "",
		})
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	http.HandleFunc("/VolumeDriver.Mount", func(w http.ResponseWriter, r *http.Request) {
		// TODO acquire containerRuntimeLock and update our state and etcd with
		// the fact that a container will soon be running on this volume...
		log.Print("<= /VolumeDriver.Mount")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatalf("Unable to read response body %s", err)
		}
		request := new(RequestMount)
		if err := json.Unmarshal(requestJSON, request); err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		filesystemId, err := procureFilesystem(name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		mountpoint, err := newContainerMountSymlink(name, filesystemId)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		// Allow things that don't want containers to start during their
		// operations to delay the start of a container. Commented out because
		// it causes a deadlock.
		/*
			state.containersLock.Lock()
			defer state.containersLock.Unlock()
		*/

		log.Printf("Mountpoint for %s: %s", name, mountpoint)
		responseJSON, _ := json.Marshal(&ResponseMount{
			Mountpoint: mountpoint,
			Err:        "",
		})
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)

		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
		go func() {
			// Do this again a second later, to cope with Docker's lack of
			// immediate consistency
			time.Sleep(time.Second)
			state.fetchRelatedContainersChan <- true
		}()
	})

	http.HandleFunc("/VolumeDriver.Unmount", func(w http.ResponseWriter, r *http.Request) {
		// TODO acquire containerRuntimeLock and update our state and etcd with
		// the fact that one less container is now running on this volume...
		writeResponseOK(w)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
		go func() {
			// Do this again a second later, to cope with Docker's lack of
			// immediate consistency
			time.Sleep(time.Second)
			state.fetchRelatedContainersChan <- true
		}()
	})

	http.HandleFunc("/VolumeDriver.List", func(w http.ResponseWriter, r *http.Request) {
		log.Print("<= /VolumeDriver.List")
		var response = ResponseList{
			Err: "",
		}

		for _, fs := range (*state).registry.Filesystems() {
			log.Printf("Mountpoint for %s: %s", fs, containerMnt(fs))
			response.Volumes = append(response.Volumes, ResponseListVolume{
				Name:       fs.StringWithoutAdmin(),
				Mountpoint: containerMnt(fs),
			})
		}

		responseJSON, _ := json.Marshal(response)
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})
	http.HandleFunc("/VolumeDriver.Get", func(w http.ResponseWriter, r *http.Request) {
		log.Print("<= /VolumeDriver.Get")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		request := new(RequestMount)
		if err := json.Unmarshal(requestJSON, request); err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		var response = ResponseGet{
			Err: "",
		}

		// Technically, fetching the TopLevelFilesystem object from the
		// registry isn't necessary, but maybe one day we'll get additional
		// Status information from that call that we want to use here, so
		// leaving it in for now rather than just hand-constructing the
		// response from the name.
		fs, err := (*state).registry.GetByName(name)
		if err != nil {
			response.Err = fmt.Sprintf("Error getting volume: %v", err)
		}

		log.Printf("Mountpoint for %s: %s", fs, containerMnt(fs.TopLevelVolume.Name))
		response.Volume = ResponseListVolume{
			Name:       fs.TopLevelVolume.Name.StringWithoutAdmin(),
			Mountpoint: containerMnt(fs.TopLevelVolume.Name),
		}

		responseJSON, _ := json.Marshal(response)
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	listener, err := net.Listen("unix", DM_SOCKET)
	if err != nil {
		log.Fatalf("Could not listen on %s: %v", DM_SOCKET, err)
	}

	http.Serve(listener, nil)
}

func (state *InMemoryState) runErrorPlugin() {
	// A variant of the normal plugin which just returns immediately with
	// errors. For bootstrapping.
	log.Print("Starting dm temporary bootstrap plugin")
	state.mustCleanupSocket()
	http.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /Plugin.Activate")
		responseJSON, _ := json.Marshal(&ResponseImplements{
			Implements: []string{"VolumeDriver"},
		})
		w.Write(responseJSON)
	})
	http.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Create")
		writeResponseErr(fmt.Errorf("I'm sorry Dave, I can't do that. I'm still starting up."), w)
	})
	http.HandleFunc("/VolumeDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Remove")
		writeResponseOK(w)
	})
	http.HandleFunc("/VolumeDriver.Path", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Path")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		request := new(RequestMount)
		if err := json.Unmarshal(requestJSON, request); err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		log.Printf("Mountpoint for %s: %s", name, containerMnt(name))
		responseJSON, _ := json.Marshal(&ResponseMount{
			Mountpoint: containerMnt(name),
			Err:        "",
		})
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
	})
	http.HandleFunc("/VolumeDriver.Mount", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Mount")
		writeResponseErr(fmt.Errorf("datamesh still starting or datamesh-etcd unable to achieve quorum"), w)
	})
	http.HandleFunc("/VolumeDriver.Unmount", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Unmount")
		writeResponseErr(fmt.Errorf("datamesh still starting or datamesh-etcd unable to achieve quorum"), w)
	})
	http.HandleFunc("/VolumeDriver.List", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.List")
		var response = ResponseList{
			Err: "datamesh still starting or datamesh-etcd unable to achieve quorum",
		}
		responseJSON, _ := json.Marshal(response)
		w.Write(responseJSON)
	})
	listener, err := net.Listen("unix", DM_SOCKET)
	if err != nil {
		log.Fatalf("Could not listen on %s: %v", DM_SOCKET, err)
	}
	http.Serve(listener, nil)
}

func writeResponseOK(w http.ResponseWriter) {
	// A shortcut to writing a ResponseOK to w
	responseJSON, _ := json.Marshal(&ResponseSimple{Err: ""})
	w.Write(responseJSON)
}

func writeResponseErr(err error, w http.ResponseWriter) {
	// A shortcut to responding with an error, and then log the error
	errString := fmt.Sprintln(err)
	log.Printf("Error: %v", err)
	responseJSON, _ := json.Marshal(&ResponseSimple{Err: errString})
	w.Write(responseJSON)
}

func (state *InMemoryState) cleanupDockerFilesystemState() error {
	err := filepath.Walk(CONTAINER_MOUNT_PREFIX, func(symlinkPath string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			target, err := os.Readlink(symlinkPath)
			log.Printf("[cleanupDockerFilesystemState] Found %s -> %s", symlinkPath, target)
			if err != nil {
				if os.IsNotExist(err) {
					// It's already gone, nothing to clean up.
				} else {
					// Some other error happened, that's not good.
					return err
				}
			} else {
				fsid, err := unmnt(target)
				log.Printf("[cleanupDockerFilesystemState] Found %s -> %s extracted fsid %s", symlinkPath, target, fsid)
				if err != nil {
					return err
				}

				deleted, err := isFilesystemDeletedInEtcd(fsid)
				if err != nil {
					return err
				}

				if deleted {
					log.Printf("[cleanupDockerFilesystemState] %s -> %s -> %s - deleting", symlinkPath, target, fsid)
					if err := os.Remove(symlinkPath); err != nil {
						return err
					}
				} else {
					// Do nothing; the symlink has been taken over by another filesystem, and points to a new non-deleted fs.
				}
			}
		}

		return nil
	})
	return err
}
