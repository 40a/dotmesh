package main

import (
	"fmt"
	"strings"
	"sync"
)

type User struct {
	Id     string
	Name   string
	Email  string
	ApiKey string
}

type SafeUser struct {
	Id    string
	Name  string
	Email string // TODO maybe this should be the hash of the email
}

type CloneWithName struct {
	Name  string
	Clone Clone
}
type ClonesList []CloneWithName

type PathToTopLevelFilesystem struct {
	TopLevelFilesystemId   string
	TopLevelFilesystemName string
	Clones                 ClonesList
}

type Clone struct {
	FilesystemId string
	Origin       Origin
}

// refers to a clone's "pointer" to a filesystem id and its snapshot.
//
// note that a clone's Origin's FilesystemId may differ from the "top level"
// filesystemId in the Registry's Clones map if the clone is attributed to a
// top-level filesystem which is *transitively* its parent but not its direct
// parent. In this case the Origin FilesystemId will always point to its direct
// parent.

type Origin struct {
	FilesystemId string
	SnapshotId   string
}

type dirtyInfo struct {
	Server     string
	DirtyBytes int64
	SizeBytes  int64
}

type containerInfo struct {
	Server     string
	Containers []DockerContainer
}

type PermissionDenied struct {
}

func (e PermissionDenied) Error() string {
	return "Permission denied."
}

type TopLevelFilesystem struct {
	TopLevelVolume DatameshVolume
	CloneVolumes   []DatameshVolume
	Owner          SafeUser
	Collaborators  []SafeUser
}

type VolumesAndClones struct {
	Volumes []TopLevelFilesystem
	Servers []Server
}

type Server struct {
	Id        string
	Addresses []string
}

type ByAddress []Server

type DatameshVolume struct {
	Id             string
	Name           string
	Clone          string
	Master         string
	SizeBytes      int64
	DirtyBytes     int64
	CommitCount    int64
	ServerStatuses map[string]string // serverId => status
}

type TransferPollResult struct {
	TransferRequestId string
	Peer              string // hostname
	User              string
	ApiKey            string
	Direction         string // "push" or "pull"

	// Hold onto this information, it might become useful for e.g. recursive
	// receives of clone filesystems.
	LocalFilesystemName  string
	LocalCloneName       string
	RemoteFilesystemName string
	RemoteCloneName      string

	// Same across both clusters
	FilesystemId string

	// TODO add clusterIds? probably comes from etcd. in fact, could be the
	// discovery id (although that is only for bootstrap... hmmm).
	InitiatorNodeId string
	PeerNodeId      string

	// XXX a Transfer that spans multiple filesystem ids won't have a unique
	// starting/target snapshot, so this is in the wrong place right now.
	// although maybe it makes sense to talk about a target *final* snapshot,
	// with interim snapshots being an implementation detail.
	StartingSnapshot string
	TargetSnapshot   string

	Index              int    // i.e. transfer 1/4 (Index=1)
	Total              int    //                   (Total=4)
	Status             string // one of "starting", "running", "finished", "error"
	NanosecondsElapsed int64
	Size               int64 // size of current segment in bytes
	Sent               int64 // number of bytes of current segment sent so far
	Message            string
}

// A container for some state that is truly global to this process.
type InMemoryState struct {
	filesystems                *fsMap
	filesystemsLock            *sync.Mutex
	myNodeId                   string
	mastersCache               *map[string]string
	mastersCacheLock           *sync.Mutex
	serverAddressesCache       *map[string]string
	serverAddressesCacheLock   *sync.Mutex
	globalSnapshotCache        *map[string]map[string][]snapshot
	globalSnapshotCacheLock    *sync.Mutex
	globalStateCache           *map[string]map[string]map[string]string
	globalStateCacheLock       *sync.Mutex
	globalContainerCache       *map[string]containerInfo
	globalContainerCacheLock   *sync.Mutex
	localReceiveProgress       *Observer
	newSnapsOnMaster           *Observer
	registry                   *Registry
	containers                 *DockerClient
	containersLock             *sync.Mutex
	fetchRelatedContainersChan chan bool
	interclusterTransfers      *map[string]TransferPollResult
	interclusterTransfersLock  *sync.Mutex
	globalDirtyCacheLock       *sync.Mutex
	globalDirtyCache           *map[string]dirtyInfo
}

type fsMap map[string]*fsMachine

// state machinery
type stateFn func(*fsMachine) stateFn

// a "filesystem machine" or "filesystem state machine"
type fsMachine struct {
	// which ZFS filesystem this statemachine is operating on
	filesystemId string
	filesystem   *filesystem
	// channel of requests going in to the state machine
	requests chan *Event
	// inner versions of the above
	innerRequests chan *Event
	// inner responses don't need to be parameterized on request id because
	// they're guaranteed to only have one goroutine reading on the channel.
	innerResponses chan *Event
	// channel of responses coming out of the state machine, indexed by request
	// id so that multiple goroutines reading responses for the same filesystem
	// id won't get the wrong result.
	responses     map[string]chan *Event
	responsesLock *sync.Mutex
	// channel notifying etcd-updater whenever snapshot state changes
	snapshotsModified chan bool
	// pointer to global state, because it's convenient to have access to it
	state *InMemoryState
	// fsMachines live forever, whereas filesystem structs do not. so
	// filesystem struct's snapshotLock can live here so that it doesn't get
	// clobbered
	snapshotsLock *sync.Mutex
	// a place to store arguments to pass to the next state
	handoffRequest *Event
	// filesystem-sliced view of new snapshot events
	newSnapsOnServers *Observer
	// current state, status field for reporting/debugging and transition observer
	currentState             string
	status                   string
	lastTransitionTimestamp  int64
	transitionObserver       *Observer
	lastTransferRequest      TransferRequest
	lastTransferRequestId    string
	externalSnapshotsChanged chan bool
	dirtyDelta               int64
	sizeBytes                int64
	lastPollResult           *TransferPollResult
}

type TransferRequest struct {
	Peer                 string // hostname
	User                 string
	ApiKey               string
	Direction            string // "push" or "pull"
	LocalFilesystemName  string
	LocalCloneName       string
	RemoteFilesystemName string
	RemoteCloneName      string
	// TODO could also include SourceSnapshot here
	TargetSnapshot string // optional, "" means "latest"
}

type EventArgs map[string]interface{}
type Event struct {
	Name string
	Args *EventArgs
}

func (ea EventArgs) String() string {
	aggr := []string{}
	for k, v := range ea {
		aggr = append(aggr, fmt.Sprintf("%s: %s", k, v))
	}
	return strings.Join(aggr, ", ")
}

func (e Event) String() string {
	return fmt.Sprintf("<Event %s: %s>", e.Name, e.Args)
}

type metadata map[string]string
type snapshot struct {
	// exported for json serialization
	Id       string
	Metadata *metadata
	// private (do not serialize)
	filesystem *filesystem
}

type filesystem struct {
	id        string
	exists    bool
	mounted   bool
	snapshots []*snapshot
	// support filesystem which is clone of another filesystem, for branching
	// purposes, with origin e.g. "<fs-uuid-of-actual-origin-snapshot>@<snap-id>"
	origin Origin
}

func castToMetadata(val interface{}) metadata {
	meta, ok := val.(metadata)
	if !ok {
		meta = metadata{}
		// massage the data into the right type
		cast := val.(map[string]interface{})
		for k, v := range cast {
			meta[k] = v.(string)
		}
	}
	return meta
}

type Prelude struct {
	SnapshotProperties []*snapshot
}

type transferFn func(
	f *fsMachine,
	fromFilesystemId, fromSnapshotId, toFilesystemId, toSnapshotId string,
	transferRequestId string, pollResult *TransferPollResult,
	client *JsonRpcClient, transferRequest *TransferRequest,
) (*Event, stateFn)
