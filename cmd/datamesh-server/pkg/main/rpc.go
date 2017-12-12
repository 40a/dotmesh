package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
	"github.com/stripe/stripe-go/sub"
)

// TODO ensure contexts are threaded through in all RPC calls for correct
// authorization.

// rpc server
type DatameshRPC struct {
	state *InMemoryState
}

func NewDatameshRPC(state *InMemoryState) *DatameshRPC {
	return &DatameshRPC{state: state}
}

func (d *DatameshRPC) Procure(
	r *http.Request, args *VolumeName, subvolume string, result *string) error {
	ctx := r.Context()
	filesystemId, err := d.state.procureFilesystem(ctx, *args)
	if err != nil {
		return err
	}
	mountpoint, err := newContainerMountSymlink(*args, filesystemId, subvolume)
	*result = mountpoint
	return err
}

func safeConfig(c Config) SafeConfig {
	safe := SafeConfig{
		Plans:           c.Plans,
		StripePublicKey: c.StripePublicKey,
	}
	return safe
}

func (d *DatameshRPC) Config(
	r *http.Request, args *struct{}, result *SafeConfig) error {
	*result = safeConfig(d.state.config)
	return nil
}

func (d *DatameshRPC) CurrentUser(
	r *http.Request, args *struct{}, result *SafeUser,
) error {

	user, err := GetUserById(r.Context().Value("authenticated-user-id").(string))
	if err != nil {
		return err
	}

	*result = safeUser(user)
	return nil
}

func (d *DatameshRPC) Get(
	r *http.Request, filesystemId *string, result *DatameshVolume) error {
	v, err := d.state.getOne(r.Context(), *filesystemId)
	if err != nil {
		return err
	}
	*result = v
	return nil
}

type PaymentDeets struct {
	Token  string
	PlanId string
}

func (d *DatameshRPC) SubmitPayment(
	r *http.Request, paymentDeets *PaymentDeets, result *bool,
) error {
	stripe.Key = d.state.config.StripePrivateKey

	user, err := GetUserById(r.Context().Value("authenticated-user-id").(string))
	if err != nil {
		return err
	}
	if user.CustomerId == "" {
		customerParams := &stripe.CustomerParams{
			Desc: fmt.Sprintf("Customer for %s", user.Email),
		}
		customerParams.SetSource(paymentDeets.Token)
		c, err := customer.New(customerParams)
		if err != nil {
			return err
		}
		user.CustomerId = c.ID
		err = user.Save()
		if err != nil {
			return err
		}
	}

	// if user.CurrentPlan != free then we're _changing_ plan, do
	// something special? currently the frontend shouldn't allow
	// this to happen (there's cancelling and subscribing; nothing
	// else)

	// create new subscription for user
	log.Printf("[SubmitPayment] Payment Deets = %+v", paymentDeets)
	s, err := sub.New(&stripe.SubParams{
		Customer: user.CustomerId,
		Items: []*stripe.SubItemsParams{
			{
				Plan: paymentDeets.PlanId,
			},
		},
	})
	if err != nil {
		return err
	}
	fmt.Sprintf("[SubmitPayment] succeeded creating subscription %v for user %v! ka-ching! :-D", s, user)

	// TODO: go talk to stripe, smash together some stuff and update the
	// current user's CustomerId. Later, Stripe will tell us about an updated
	// CurrentPlan.
	*result = true

	return nil
}

// List all filesystems in the cluster.
func (d *DatameshRPC) List(
	r *http.Request, args *struct{}, result *map[string]map[string]DatameshVolume) error {
	log.Printf("[List] starting!")

	d.state.mastersCacheLock.Lock()
	filesystems := []string{}
	for fs, _ := range *d.state.mastersCache {
		filesystems = append(filesystems, fs)
	}
	d.state.mastersCacheLock.Unlock()

	gather := map[string]map[string]DatameshVolume{}
	for _, fs := range filesystems {
		one, err := d.state.getOne(r.Context(), fs)
		// Just skip this in the result list if the context (eg authenticated
		// user) doesn't have permission to read it.
		if err != nil {
			switch err := err.(type) {
			default:
				log.Printf("[List] err: %v", err)
				return err
			case PermissionDenied:
				log.Printf("[List] permission denied reading %v", fs)
				continue
			}
		}
		submap, ok := gather[one.Name.Namespace]
		if !ok {
			submap = map[string]DatameshVolume{}
			gather[one.Name.Namespace] = submap
		}

		submap[one.Name.Name] = one
	}
	log.Printf("[List] gather = %+v", gather)
	*result = gather
	return nil
}

func (d *DatameshRPC) Create(
	r *http.Request, filesystemName *VolumeName, result *bool) error {
	_, ch, err := d.state.CreateFilesystem(r.Context(), filesystemName)
	if err != nil {
		return err
	}
	e := <-ch
	if e.Name != "created" {
		return fmt.Errorf(
			"Could not create volume %s: unexpected response %s - %s",
			filesystemName, e.Name, e.Args,
		)
	}
	return nil
}

// Switch any containers which are currently using the given volume and clone
// name so that they use the new clone name by stopping them, changing the
// symlink, and starting them again.
func (d *DatameshRPC) SwitchContainers(
	r *http.Request,
	args *struct{ Namespace, TopLevelFilesystemName, CurrentCloneName, NewCloneName string },
	result *bool,
) error {
	toFilesystemId, err := d.state.registry.MaybeCloneFilesystemId(
		VolumeName{args.Namespace, args.TopLevelFilesystemName},
		args.NewCloneName,
	)
	if err != nil {
		return err
	}

	// Stop any other containers getting started until this function completes
	d.state.containersLock.Lock()
	defer d.state.containersLock.Unlock()

	// TODO Maybe be a bit more selective about which containers we stop/start
	// here (only ones which are using the given volume *and branch* name).
	if err := d.state.containers.Stop(args.TopLevelFilesystemName); err != nil {
		return err
	}
	err = d.state.containers.SwitchSymlinks(
		args.TopLevelFilesystemName,
		mnt(toFilesystemId),
	)
	if err != nil {
		// TODO try to rollback (run Start)
		return err
	}
	return d.state.containers.Start(args.TopLevelFilesystemName)
}

// Containers that were recently known to be running on a given filesystem.
func (d *DatameshRPC) Containers(
	r *http.Request,
	args *struct{ Namespace, TopLevelFilesystemName, CloneName string },
	result *[]DockerContainer,
) error {
	log.Printf("[Containers] called with %+v", *args)
	filesystemId, err := d.state.registry.MaybeCloneFilesystemId(
		VolumeName{args.Namespace, args.TopLevelFilesystemName},
		args.CloneName,
	)
	if err != nil {
		log.Printf("[Containers] died of %#v", err)
		return err
	}
	d.state.globalContainerCacheLock.Lock()
	defer d.state.globalContainerCacheLock.Unlock()
	containerInfo, ok := (*d.state.globalContainerCache)[filesystemId]
	if !ok {
		*result = []DockerContainer{}
		return nil
	}
	// TODO maybe check that the server this containerInfo pertains to matches
	// what we believe the current master is, and otherwise flag to the
	// consumer of the API that the data may be stale
	*result = containerInfo.Containers
	return nil
}

// Containers that were recently known to be running on a given filesystem.
func (d *DatameshRPC) ContainersById(
	r *http.Request,
	filesystemId *string,
	result *[]DockerContainer,
) error {
	d.state.globalContainerCacheLock.Lock()
	defer d.state.globalContainerCacheLock.Unlock()
	containerInfo, ok := (*d.state.globalContainerCache)[*filesystemId]
	if !ok {
		*result = []DockerContainer{}
		return nil
	}
	*result = containerInfo.Containers
	return nil
}

func (d *DatameshRPC) Exists(
	r *http.Request,
	args *struct{ Namespace, TopLevelFilesystemName, CloneName string },
	result *string,
) error {
	*result = d.state.registry.Exists(VolumeName{args.Namespace, args.TopLevelFilesystemName}, args.CloneName)
	return nil
}

func (d *DatameshRPC) Lookup(
	r *http.Request,
	args *struct{ Namespace, TopLevelFilesystemName, CloneName string },
	result *string,
) error {
	filesystemId, err := d.state.registry.MaybeCloneFilesystemId(
		VolumeName{args.Namespace, args.TopLevelFilesystemName}, args.CloneName,
	)
	if err != nil {
		return err
	}
	*result = filesystemId
	return nil
}

// Get a list of snapshots for a filesystem (or its specified clone). Snapshot
// objects have "id" and "metadata" fields, where id is an opaque, unique
// string and metadata is a mapping from strings to strings.
func (d *DatameshRPC) Snapshots(
	r *http.Request,
	args *struct{ Namespace, TopLevelFilesystemName, CloneName string },
	result *[]snapshot,
) error {
	filesystemId, err := d.state.registry.MaybeCloneFilesystemId(
		VolumeName{args.Namespace, args.TopLevelFilesystemName},
		args.CloneName,
	)
	if err != nil {
		return err
	}
	snapshots, err := d.state.snapshotsForCurrentMaster(filesystemId)
	if err != nil {
		return err
	}
	*result = snapshots
	return nil
}

func (d *DatameshRPC) SnapshotsById(
	r *http.Request,
	filesystemId *string,
	result *[]snapshot,
) error {
	snapshots, err := d.state.snapshotsForCurrentMaster(*filesystemId)
	if err != nil {
		return err
	}
	*result = snapshots
	return nil
}

// Acknowledge that an authenticated connection had been successfully established.
func (d *DatameshRPC) Ping(r *http.Request, args *struct{}, result *bool) error {
	*result = true
	return nil
}

// Take a snapshot of a specific filesystem on the master.
func (d *DatameshRPC) Snapshot(
	r *http.Request, args *struct{ Namespace, TopLevelFilesystemName, CloneName, Message string },
	result *bool,
) error {
	// Insert a command into etcd for the current master to respond to, and
	// wait for a response to be inserted into etcd as well, before firing with
	// that.
	filesystemId, err := d.state.registry.MaybeCloneFilesystemId(
		VolumeName{args.Namespace, args.TopLevelFilesystemName},
		args.CloneName,
	)
	if err != nil {
		return err
	}
	// NB: metadata keys must always start lowercase, because zfs
	user, _, _ := r.BasicAuth()
	meta := metadata{"message": args.Message, "author": user}

	responseChan, err := d.state.globalFsRequest(
		filesystemId,
		&Event{Name: "snapshot",
			Args: &EventArgs{"metadata": meta}},
	)
	if err != nil {
		// meh, maybe REST *would* be nicer
		return err
	}

	// TODO this may never succeed, if the master for it never shows up. maybe
	// this response should have a timeout associated with it.
	e := <-responseChan
	if e.Name == "snapshotted" {
		log.Printf("Snapshotted %s", filesystemId)
		*result = true
	} else {
		return maybeError(e)
	}
	return nil
}

// Rollback a specific filesystem to the specified snapshot_id on the master.
func (d *DatameshRPC) Rollback(
	r *http.Request,
	args *struct{ Namespace, TopLevelFilesystemName, CloneName, SnapshotId string },
	result *bool,
) error {
	// Insert a command into etcd for the current master to respond to, and
	// wait for a response to be inserted into etcd as well, before firing with
	// that.
	filesystemId, err := d.state.registry.MaybeCloneFilesystemId(
		VolumeName{args.Namespace, args.TopLevelFilesystemName},
		args.CloneName,
	)
	if err != nil {
		return err
	}
	responseChan, err := d.state.globalFsRequest(
		filesystemId,
		&Event{Name: "rollback",
			Args: &EventArgs{"rollbackTo": args.SnapshotId}},
	)
	if err != nil {
		return err
	}

	// TODO this may never succeed, if the master for it never shows up. maybe
	// this response should have a timeout associated with it.
	e := <-responseChan
	if e.Name == "rolled-back" {
		log.Printf(
			"Rolled back %s@%s to %s",
			args.TopLevelFilesystemName,
			args.CloneName,
			args.SnapshotId,
		)
		*result = true
	} else {
		return maybeError(e)
	}
	return nil
}

func maybeError(e *Event) error {
	log.Printf("Unexpected response %s - %s", e.Name, e.Args)
	err, ok := (*e.Args)["err"]
	if ok {
		return err.(error)
	} else {
		return fmt.Errorf("Unexpected response %s - %s", e.Name, e.Args)
	}
}

// Return a list of clone names attributed to a given top-level filesystem name
func (d *DatameshRPC) Clones(r *http.Request, filesystemName *VolumeName, result *[]string) error {
	filesystemId, err := d.state.registry.IdFromName(*filesystemName)
	if err != nil {
		return err
	}
	filesystems := d.state.registry.ClonesFor(filesystemId)
	names := []string{}
	for name, _ := range filesystems {
		names = append(names, name)
	}
	sort.Strings(names)
	*result = names
	return nil
}

func (d *DatameshRPC) Clone(
	r *http.Request,
	args *struct{ Namespace, Volume, SourceBranch, NewBranchName, SourceSnapshotId string },
	result *bool,
) error {
	// TODO pass through to a globalFsRequest

	// find the real origin filesystem we're trying to clone from, identified
	// to the user by "volume + sourcebranch", but to us by an underlying
	// filesystem id (could be a clone of a clone)

	// NB: are we special-casing master here? Yes, I think. You'll never be
	// able to delete the master branch because it's equivalent to the
	// topLevelFilesystemId. You could rename it though, I suppose. That's
	// probably fine. We could fix this later by allowing promotions.

	tlf, err := d.state.registry.LookupFilesystem(VolumeName{args.Namespace, args.Volume})
	if err != nil {
		return err
	}
	var originFilesystemId string

	// find whether branch refers to top-level fs or a clone, by guessing based
	// on name convention. XXX this shouldn't be dealing with "master" and
	// branches
	if args.SourceBranch == DEFAULT_BRANCH {
		originFilesystemId = tlf.TopLevelVolume.Id
	} else {
		clone, err := d.state.registry.LookupClone(
			tlf.TopLevelVolume.Id, args.SourceBranch,
		)
		originFilesystemId = clone.FilesystemId
		if err != nil {
			return err
		}
	}
	// target node is responsible for creating registry entry (so that they're
	// as close as possible to eachother), so give it all the info it needs to
	// do that.
	responseChan, err := d.state.globalFsRequest(
		originFilesystemId,
		&Event{Name: "clone",
			Args: &EventArgs{
				"topLevelFilesystemId": tlf.TopLevelVolume.Id,
				"originFilesystemId":   originFilesystemId,
				"originSnapshotId":     args.SourceSnapshotId,
				"newBranchName":        args.NewBranchName,
			},
		},
	)
	if err != nil {
		return err
	}

	// TODO this may never succeed, if the master for it never shows up. maybe
	// this response should have a timeout associated with it.
	e := <-responseChan
	if e.Name == "cloned" {
		log.Printf(
			"Cloned %s:%s@%s (%s) to %s", args.Volume,
			args.SourceBranch, args.SourceSnapshotId, originFilesystemId,
		)
		*result = true
	} else {
		return maybeError(e)
	}
	return nil
}

// Return local version information.
func (d *DatameshRPC) Version(
	r *http.Request, args *struct{}, result *map[string]string) error {
	*result = map[string]string{
		"Version": "0.1", "Name": "Datamesh", "Website": "https://datamesh.io",
	}
	return nil
}

func (d *DatameshRPC) registerFilesystemBecomeMaster(
	ctx context.Context,
	filesystemNamespace, filesystemName, cloneName, filesystemId string,
	path PathToTopLevelFilesystem,
) error {
	log.Printf("[registerFilesystemBecomeMaster] called: filesystemNamespace=%s, filesystemName=%s, cloneName=%s, filesystemId=%s path=%+v",
		filesystemNamespace, filesystemName, cloneName, filesystemId, path)
	// ensure there's a filesystem machine for it (and its parents), otherwise
	// it won't process any events. in the case where it already exists, this
	// is a noop.
	log.Printf("[registerFilesystemBecomeMaster] calling initFilesystemMachine for %s", filesystemId)
	d.state.initFilesystemMachine(filesystemId)
	log.Printf("[registerFilesystemBecomeMaster] done initFilesystemMachine for %s", filesystemId)

	kapi, err := getEtcdKeysApi()
	if err != nil {
		return err
	}
	filesystemIds := []string{path.TopLevelFilesystemId}
	for _, c := range path.Clones {
		filesystemIds = append(filesystemIds, c.Clone.FilesystemId)
	}
	for _, f := range filesystemIds {
		_, err := kapi.Get(
			context.Background(),
			fmt.Sprintf(
				"%s/filesystems/masters/%s", ETCD_PREFIX, f,
			),
			nil,
		)
		if !client.IsKeyNotFound(err) && err != nil {
			return err
		}
		if err != nil {
			// TODO: maybe check value, and if it's != me, raise an error?
			// key doesn't already exist
			_, err = kapi.Set(
				context.Background(),
				fmt.Sprintf(
					"%s/filesystems/masters/%s", ETCD_PREFIX, f,
				),
				// i pick -- me!
				// TODO maybe one day pick the node with the most disk space or
				// something
				d.state.myNodeId,
				// only pick myself as current master if no one else has it
				&client.SetOptions{PrevExist: client.PrevNoExist},
			)
			if err != nil {
				return err
			}
		}
	}

	// do this after, in case filesystemId already existed above
	// use path to set up requisite clone metadata

	// set up top level filesystem first, if not exists
	if d.state.registry.Exists(path.TopLevelFilesystemName, "") == "" {
		err = d.state.registry.RegisterFilesystem(
			ctx, path.TopLevelFilesystemName, path.TopLevelFilesystemId,
		)
		if err != nil {
			return err
		}
	}

	// for each clone, set up clone
	for _, c := range path.Clones {
		err = d.state.registry.RegisterClone(c.Name, path.TopLevelFilesystemId, c.Clone)
		if err != nil {
			return err
		}
	}

	log.Printf(
		"[registerFilesystemBecomeMaster] set master and registered fs in registry for %s",
		filesystemId,
	)
	return nil
}

func (d *DatameshRPC) RegisterFilesystem(
	r *http.Request,
	args *struct {
		Namespace, TopLevelFilesystemName, CloneName, FilesystemId string
		PathToTopLevelFilesystem                                   PathToTopLevelFilesystem
		BecomeMasterIfNotExists                                    bool
	},
	result *bool,
) error {
	log.Printf("[RegisterFilesystem] called with args: %+v", args)

	isAdmin, err := AuthenticatedUserIsNamespaceAdministrator(r.Context(), args.Namespace)
	if err != nil {
		return err
	}

	if !isAdmin {
		return fmt.Errorf("User is not an administrator for namespace %s, so cannot create volumes",
			args.Namespace)
	}

	if !args.BecomeMasterIfNotExists {
		panic("can't not become master in RegisterFilesystem inter-cluster rpc")
	}
	err = d.registerFilesystemBecomeMaster(
		r.Context(),
		args.Namespace,
		args.TopLevelFilesystemName,
		args.CloneName,
		args.FilesystemId,
		args.PathToTopLevelFilesystem,
	)
	*result = true
	return err
}

func (d *DatameshRPC) GetTransfer(
	r *http.Request,
	args *string,
	result *TransferPollResult,
) error {
	// Poll the status of a transfer by fetching it from our local cache.
	res, ok := (*d.state.interclusterTransfers)[*args]
	if !ok {
		return fmt.Errorf("No such intercluster transfer %s", *args)
	}
	*result = res
	return nil
}

// Register a transfer from an initiator (the cluster where the user initially
// connected) to a peer (the cluster which will be the target of a push/pull).
func (d *DatameshRPC) RegisterTransfer(
	r *http.Request,
	args *TransferPollResult,
	result *bool,
) error {
	log.Printf("[RegisterTransfer] called with args: %+v", args)
	serialized, err := json.Marshal(args)
	if err != nil {
		return err
	}
	kapi, err := getEtcdKeysApi()
	if err != nil {
		return err
	}

	_, err = kapi.Set(
		context.Background(),
		fmt.Sprintf(
			"%s/filesystems/transfers/%s", ETCD_PREFIX, args.TransferRequestId,
		),
		string(serialized),
		nil,
	)
	if err != nil {
		return err
	}
	// XXX A transfer should be able to span multiple filesystemIds, really. So
	// tying a transfer to a filesystem id is probably wrong. except, the thing
	// being updated is a specific branch (filesystem id), it's ok if it drags
	// dependent snapshots along with it.
	_, err = d.state.globalFsRequest(args.FilesystemId, &Event{
		Name: "peer-transfer",
		Args: &EventArgs{
			"Transfer": args,
		},
	})
	if err != nil {
		return err
	}
	/*
		// XXX should we be throwing away a result? not doing so probably leaks
		// goroutines.
		go func() {
			// asynchronously throw away the response, transfers can be polled via
			// their own entries in etcd
			e := <-f.responses // XXX is this right???
			log.Printf("finished peer-transfer of %s, %s", args, e)
		}()
	*/
	return nil
}

// Need both push and pull because one cluster will often be behind NAT.
// Transfer will immediately return a transferId which can be queried until
// completion
func (d *DatameshRPC) Transfer(
	r *http.Request,
	args *TransferRequest,
	result *string,
) error {
	client := NewJsonRpcClient(args.User, args.Peer, args.ApiKey)

	log.Printf("[Transfer] starting with %+v", safeArgs(*args))

	var remoteFilesystemId string
	err := client.CallRemote(r.Context(),
		"DatameshRPC.Exists", map[string]string{
			"Namespace":              args.RemoteNamespace,
			"TopLevelFilesystemName": args.RemoteFilesystemName,
			"CloneName":              args.RemoteCloneName,
		}, &remoteFilesystemId)
	if err != nil {
		return err
	}

	localFilesystemId := d.state.registry.Exists(
		VolumeName{args.LocalNamespace, args.LocalFilesystemName}, args.LocalCloneName,
	)

	remoteExists := remoteFilesystemId != ""
	localExists := localFilesystemId != ""

	if !remoteExists && !localExists {
		return fmt.Errorf("Both local and remote filesystems don't exist. %+v", args)
	}
	if args.Direction == "push" && !localExists {
		return fmt.Errorf("Can't push when local doesn't exist")
	}
	if args.Direction == "pull" && !remoteExists {
		return fmt.Errorf("Can't pull when remote doesn't exist")
	}

	var localPath, remotePath PathToTopLevelFilesystem
	if args.Direction == "push" {
		localPath, err = d.state.registry.deducePathToTopLevelFilesystem(
			VolumeName{args.LocalNamespace, args.LocalFilesystemName}, args.LocalCloneName,
		)
		if err != nil {
			return fmt.Errorf(
				"Can't deduce path to top level filesystem for %s/%s,%s: %s",
				args.LocalNamespace, args.LocalFilesystemName, args.LocalCloneName, err,
			)
		}

		// Path is the same on the remote, except with a potentially different name
		remotePath = localPath
		remotePath.TopLevelFilesystemName = VolumeName{args.RemoteNamespace, args.RemoteFilesystemName}
	} else if args.Direction == "pull" {
		err := client.CallRemote(r.Context(),
			"DatameshRPC.DeducePathToTopLevelFilesystem", map[string]interface{}{
				"RemoteNamespace":      args.RemoteNamespace,
				"RemoteFilesystemName": args.RemoteFilesystemName,
				"RemoteCloneName":      args.RemoteCloneName,
			},
			&remotePath,
		)
		if err != nil {
			return fmt.Errorf(
				"Can't deduce path to top level filesystem for %s/%s,%s: %s",
				args.RemoteNamespace, args.RemoteFilesystemName, args.RemoteCloneName, err,
			)
		}
		// Path is the same locally, except with a potentially different name
		localPath = remotePath
		localPath.TopLevelFilesystemName = VolumeName{args.LocalNamespace, args.LocalFilesystemName}
	}

	log.Printf("[Transfer] got paths: local=%+v remote=%+v", localPath, remotePath)

	var filesystemId string
	if args.Direction == "push" && !remoteExists {
		// pre-create the remote registry entry and pick a master for it to
		// land on on the remote
		var result bool

		err := client.CallRemote(r.Context(),
			"DatameshRPC.RegisterFilesystem", map[string]interface{}{
				"Namespace":              args.RemoteNamespace,
				"TopLevelFilesystemName": args.RemoteFilesystemName,
				"CloneName":              args.RemoteCloneName,
				"FilesystemId":           localFilesystemId,
				// record that you are the master if the fs doesn't exist yet, so
				// that you can receive a push. This should cause an fsMachine to
				// get spawned on this node, listening out for globalFsRequests for
				// this filesystemId on that cluster.
				"BecomeMasterIfNotExists":  true,
				"PathToTopLevelFilesystem": remotePath,
			}, &result)
		if err != nil {
			return err
		}
		filesystemId = localFilesystemId
	} else if args.Direction == "pull" && !localExists {
		// pre-create the local registry entry and pick a master for it to land
		// on locally (me!)
		err = d.registerFilesystemBecomeMaster(
			r.Context(),
			args.LocalNamespace,
			args.LocalFilesystemName,
			args.LocalCloneName,
			remoteFilesystemId,
			localPath,
		)
		if err != nil {
			return err
		}
		filesystemId = remoteFilesystemId
	} else if remoteExists && localExists && remoteFilesystemId != localFilesystemId {
		return fmt.Errorf(
			"Cannot reconcile filesystems with different ids, remote=%s, local=%s, args=%+v",
			remoteFilesystemId, localFilesystemId, safeArgs(*args),
		)
	} else if remoteExists && localExists && remoteFilesystemId == localFilesystemId {
		filesystemId = localFilesystemId

		// This is an incremental update, not a new filesystem for the writer.
		// Check whether there are uncommitted changes or containers running
		// where the writes are going to happen.

		var cs []DockerContainer
		var dirtyBytes int64
		// TODO Add a check that the filesystem hasn't diverged snapshot-wise.

		if args.Direction == "push" {
			// Ask the remote
			var v DatameshVolume
			err := client.CallRemote(r.Context(), "DatameshRPC.Get", filesystemId, &v)
			if err != nil {
				return err
			}
			log.Printf("[TransferIt] for %s, got datamesh volume: %s", filesystemId, v)
			dirtyBytes = v.DirtyBytes
			log.Printf("[TransferIt] got %d dirty bytes for %s from peer", dirtyBytes, filesystemId)

			err = client.CallRemote(r.Context(), "DatameshRPC.ContainersById", filesystemId, &cs)
			if err != nil {
				return err
			}

		} else if args.Direction == "pull" {
			// Consult ourselves
			v, err := d.state.getOne(r.Context(), filesystemId)
			if err != nil {
				return err
			}
			dirtyBytes = v.DirtyBytes
			log.Printf("[TransferIt] got %d dirty bytes for %s from local", dirtyBytes, filesystemId)

			d.state.globalContainerCacheLock.Lock()
			defer d.state.globalContainerCacheLock.Unlock()
			c, _ := (*d.state.globalContainerCache)[filesystemId]
			cs = c.Containers
		}

		if dirtyBytes > 0 {
			return fmt.Errorf(
				"Aborting because there are %.2f MiB of uncommitted changes on volume "+
					"where data would be written. Use 'dm reset' to roll back.",
				float64(dirtyBytes)/(1024*1024),
			)
		}

		if len(cs) > 0 {
			containersRunning := []string{}
			for _, c := range cs {
				containersRunning = append(containersRunning, string(c.Name))
			}
			return fmt.Errorf(
				"Aborting because there are active containers running on "+
					"volume where data would be written: %s. Stop the containers.",
				strings.Join(containersRunning, ", "),
			)
		}

	} else {
		return fmt.Errorf(
			"Unexpected combination of factors: "+
				"remoteExists: %t, localExists: %t, "+
				"remoteFilesystemId: %s, localFilesystemId: %s",
			remoteExists, localExists, remoteFilesystemId, localFilesystemId,
		)
	}

	// Now run globalFsRequest, returning the request id, to make the master of
	// a (possibly nonexisting) filesystem start pulling or pushing it, and
	// make it update status as it goes in a new pollable "transfers" object in
	// etcd.

	responseChan, requestId, err := d.state.globalFsRequestId(
		filesystemId,
		&Event{Name: "transfer",
			Args: &EventArgs{
				"Transfer": args,
			},
		},
	)
	if err != nil {
		return err
	}
	go func() {
		// asynchronously throw away the response, transfers can be polled via
		// their own entries in etcd
		e := <-responseChan
		log.Printf("finished transfer of %+v, %+v", args, e)
	}()

	*result = requestId
	return nil
}

func safeArgs(t TransferRequest) TransferRequest {
	t.ApiKey = "<redacted>"
	return t
}

func (a ByAddress) Len() int      { return len(a) }
func (a ByAddress) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByAddress) Less(i, j int) bool {
	if len(a[i].Addresses) == 0 || len(a[j].Addresses) == 0 {
		return false
	}
	return a[i].Addresses[0] < a[j].Addresses[0]
}

// Return data showing all volumes, their clones, along with information about
// them such as the current state of their state machines on each server, etc.
//
// TODO should this function be the same as List?
func (d *DatameshRPC) AllVolumesAndClones(
	r *http.Request,
	args *struct{},
	result *VolumesAndClones,
) error {
	log.Printf("[AllVolumesAndClones] starting...")

	vac := VolumesAndClones{}

	d.state.serverAddressesCacheLock.Lock()
	for server, addresses := range *d.state.serverAddressesCache {
		vac.Servers = append(vac.Servers, Server{
			Id: server, Addresses: strings.Split(addresses, ","),
		})
	}
	d.state.serverAddressesCacheLock.Unlock()
	sort.Sort(ByAddress(vac.Servers))

	filesystemNames := d.state.registry.Filesystems()
	for _, fsName := range filesystemNames {
		tlfId, err := d.state.registry.IdFromName(fsName)
		if err != nil {
			return err
		}
		// XXX: crappyTlf is crappy because it contains an incomplete
		// TopLevelFilesystem object (see UpdateFilesystemFromEtcd). The only
		// thing we use it for here is the owner and collaborator data, and we
		// construct a new TopLevelFilesystem for ourselves. Probably the
		// following logic should be put somewhere inside the registry...
		crappyTlf, err := d.state.registry.LookupFilesystem(fsName)
		if err != nil {
			return err
		}
		tlf := TopLevelFilesystem{}
		/*
			TopLevelVolume DatameshVolume
			CloneVolumes   []DatameshVolume
			Owner          User
			Collaborators  []User
		*/
		v, err := d.state.getOne(r.Context(), tlfId)
		// Just skip this in the result list if the context (eg authenticated
		// user) doesn't have permission to read it.
		if err != nil {
			switch err := err.(type) {
			default:
				log.Printf("[AllVolumesAndClones] ERROR in getOne(%v): %v, continuing...", tlfId, err)
				continue
			case PermissionDenied:
				continue
			}
		}
		tlf.TopLevelVolume = v
		// now add clones to tlf
		clones := d.state.registry.ClonesFor(tlfId)
		cloneNames := []string{}
		for c, _ := range clones {
			cloneNames = append(cloneNames, c)
		}
		sort.Strings(cloneNames)
		for _, cloneName := range cloneNames {
			clone := clones[cloneName]
			c, err := d.state.getOne(r.Context(), clone.FilesystemId)
			if err != nil {
				return err
			}
			tlf.CloneVolumes = append(tlf.CloneVolumes, c)
		}
		tlf.Owner = crappyTlf.Owner
		tlf.Collaborators = crappyTlf.Collaborators
		vac.Volumes = append(vac.Volumes, tlf)
	}
	*result = vac
	log.Printf("[AllVolumesAndClones] finished!")
	return nil
}

func (d *DatameshRPC) AddCollaborator(
	r *http.Request,
	args *struct {
		Volume       string
		Collaborator string
	},
	result *bool,
) error {
	// check authenticated user is owner of volume.
	crappyTlf, clone, err := d.state.registry.LookupFilesystemById(args.Volume)
	if err != nil {
		return err
	}
	if clone != "" {
		return fmt.Errorf(
			"Please add collaborators to top level filesystem, not clone",
		)
	}
	authorized, err := crappyTlf.AuthorizeOwner(r.Context())
	if err != nil {
		return err
	}
	if !authorized {
		return fmt.Errorf(
			"Not owner. Please ask the owner to add the collaborator.",
		)
	}
	// add collaborator in registry, re-save.
	potentialCollaborator, err := GetUserByName(args.Collaborator)
	if err != nil {
		return err
	}
	newCollaborators := append(crappyTlf.Collaborators, safeUser(potentialCollaborator))
	err = d.state.registry.UpdateCollaborators(r.Context(), crappyTlf, newCollaborators)
	if err != nil {
		return err
	}
	*result = true
	return nil
}

func (d *DatameshRPC) DeducePathToTopLevelFilesystem(
	r *http.Request,
	args *struct {
		RemoteNamespace      string
		RemoteFilesystemName string
		RemoteCloneName      string
	},
	result *PathToTopLevelFilesystem,
) error {
	log.Printf("[DeducePathToTopLevelFilesystem] called with args: %+v", args)
	res, err := d.state.registry.deducePathToTopLevelFilesystem(
		VolumeName{args.RemoteNamespace, args.RemoteFilesystemName}, args.RemoteCloneName,
	)
	if err != nil {
		return err
	}
	*result = res
	log.Printf("[DeducePathToTopLevelFilesystem] succeeded: args %+v -> result %+v", args, res)
	return nil
}

func (d *DatameshRPC) PredictSize(
	r *http.Request,
	args *struct {
		FromFilesystemId string
		FromSnapshotId   string
		ToFilesystemId   string
		ToSnapshotId     string
	},
	result *int64,
) error {
	log.Printf("[PredictSize] got args %+v", args)
	size, err := predictSize(
		args.FromFilesystemId, args.FromSnapshotId, args.ToFilesystemId, args.ToSnapshotId,
	)
	if err != nil {
		return err
	}
	*result = size
	return nil
}

func checkNotInUse(d *DatameshRPC, fsid string, origins map[string]string) error {
	containersInUse := func() int {
		d.state.globalContainerCacheLock.Lock()
		defer d.state.globalContainerCacheLock.Unlock()
		containerInfo, ok := (*d.state.globalContainerCache)[fsid]
		if !ok {
			return 0
		}
		return len(containerInfo.Containers)
	}()
	if containersInUse > 0 {
		return fmt.Errorf("We cannot delete the volume %s when %d containers are still using it", fsid, containersInUse)
	}
	for child, parent := range origins {
		if parent == fsid {
			err := checkNotInUse(d, child, origins)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func sortFilesystemsInDeletionOrder(in []string, rootId string, origins map[string]string) []string {
	// Recursively zap any children
	for child, parent := range origins {
		if parent == rootId {
			in = sortFilesystemsInDeletionOrder(in, child, origins)
		}
	}
	// Then zap the root
	in = append(in, rootId)
	return in
}

func (d *DatameshRPC) DeleteVolume(
	r *http.Request,
	args *VolumeName,
	result *bool,
) error {
	*result = false

	user, err := GetUserById(r.Context().Value("authenticated-user-id").(string))

	// Look up the top-level filesystem. This will error if the
	// filesystem name isn't registered.
	filesystem, err := d.state.registry.LookupFilesystem(*args)
	if err != nil {
		return err
	}

	authorized, err := filesystem.AuthorizeOwner(r.Context())
	if err != nil {
		return err
	}
	if !authorized {
		return fmt.Errorf(
			"You are not the owner of volume %s/%s. Only the owner can delete it.",
			args.Namespace, args.Name,
		)

	}

	// Find the list of all clones of the filesystem, as we need to delete each independently.
	filesystems := d.state.registry.ClonesFor(filesystem.TopLevelVolume.Id)

	// We can't destroy a filesystem that's an origin for another
	// filesystem, so let's topologically sort them and destroy them leaves-first.

	// Analyse the list of filesystems, putting it into a more useful form for our purposes
	origins := make(map[string]string)
	names := make(map[string]string)
	for name, fs := range filesystems {
		// Record the origin
		origins[fs.FilesystemId] = fs.Origin.FilesystemId
		// Record the name
		names[fs.FilesystemId] = name
	}

	// FUTURE WORK: If we ever need to delete just some clones, we
	// can do so by picking a different rootId here. See
	// https://github.com/datamesh-io/datamesh/issues/58
	rootId := filesystem.TopLevelVolume.Id

	// Check all clones are not in use. This is no guarantee one won't
	// come into use while we're processing the deletion, but it's nice
	// for the user to try and check first.

	err = checkNotInUse(d, rootId, origins)
	if err != nil {
		return err
	}

	filesystemsInOrder := make([]string, 0)
	filesystemsInOrder = sortFilesystemsInDeletionOrder(filesystemsInOrder, rootId, origins)

	// What if we are interrupted during this loop?

	// Because we delete from the leaves up, we SHOULD be OK: the
	// system may end up in a state where some clones are gone, but
	// the top-level filesystem remains and a new deletion on that
	// should pick up where we left off.  I don't know how to easily
	// test that with the current test harness, however, so here's
	// hoping I'm right.
	for _, fsid := range filesystemsInOrder {
		// At this point, check the filesystem has no containers
		// using it and error if so, for usability. This does not mean the
		// filesystem is unused from here onwards, as it could come into
		// use at any point.

		// This will error if the filesystem is already marked as
		// deleted; it shouldn't be in the metadata if it was, so
		// hopefully that will never happen.
		if filesystem.TopLevelVolume.Id == fsid {
			// master clone, so record the name to delete and no clone registry entry to delete
			err = d.state.markFilesystemAsDeletedInEtcd(fsid, user.Name, *args, "", "")
		} else {
			// Not the master clone, so don't record a name to delete, but do record a clone name for deletion
			err = d.state.markFilesystemAsDeletedInEtcd(
				fsid, user.Name, VolumeName{},
				filesystem.TopLevelVolume.Id, names[fsid])
		}
		if err != nil {
			return err
		}

		// Block until the filesystem is gone locally (it may still be
		// dying on other nodes in the cluster, but it's too costly to
		// track that for the gains it gives us)
		waitForFilesystemDeath(fsid)

		// As we only block for completion locally, there IS a chance
		// that the deletions will happen in the wrong order on other
		// nodes in the cluster.  This may mean that some of them fail
		// with an error, because their origins still exist.  However,
		// hopefully, the discovery-triggers-redeletion code will cause
		// them to eventually be deleted.
	}

	// If we're deleting the entire filesystem rather than just a
	// clone, we need to unregister it.

	// At this point, we have an inconsistent system state: the clone
	// filesystems are marked for deletion, but their name is still
	// registered in the registry. If we crash here, the name is taken
	// by a nonexistant filesystem.

	// This, however, is then recovered from by the
	// cleanupDeletedFilesystems function, which is invoked
	// periodically.

	if rootId == filesystem.TopLevelVolume.Id {
		err = d.state.registry.UnregisterFilesystem(*args)
		if err != nil {
			return err
		}
	}

	*result = true
	return nil
}
