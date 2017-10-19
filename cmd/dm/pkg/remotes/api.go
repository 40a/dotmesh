package remotes

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	"gopkg.in/cheggaaa/pb.v1"
)

const DEFAULT_BRANCH string = "master"

type DatameshAPI struct {
	Configuration *Configuration
	configPath    string
	client        *JsonRpcClient
}

type DatameshVolume struct {
	Id          string
	Name        VolumeName
	Clone       string
	Master      string
	SizeBytes   int64
	DirtyBytes  int64
	CommitCount int64
}

func CheckName(name string) bool {
	// TODO add more checks around sensible names?
	return len(name) <= 50
}

func NewDatameshAPI(configPath string) (*DatameshAPI, error) {
	c, err := NewConfiguration(configPath)
	if err != nil {
		return nil, err
	}
	client, err := c.ClusterFromCurrentRemote()
	// intentionally disregard err, since not being able to get a client is
	// non-fatal for some operations (like creating a remote). instead, push
	// the error checking into CallRemote.
	d := &DatameshAPI{
		Configuration: c,
		client:        client,
	}
	return d, nil
}

// proxy thru
func (dm *DatameshAPI) CallRemote(
	ctx context.Context, method string, args interface{}, response interface{},
) error {
	return dm.client.CallRemote(ctx, method, args, response)
}

func (dm *DatameshAPI) Ping() (bool, error) {
	var response bool
	err := dm.client.CallRemote(context.Background(), "DatameshRPC.Ping", struct{}{}, &response)
	if err != nil {
		return false, err
	}
	return response, nil
}

func (dm *DatameshAPI) NewVolume(volumeName string) error {
	var response bool
	namespace, name, err := ParseNamespacedVolume(volumeName)
	if err != nil {
		return err
	}
	sendVolumeName := VolumeName{
		Namespace: namespace,
		Name:      name,
	}
	err = dm.client.CallRemote(context.Background(), "DatameshRPC.Create", sendVolumeName, &response)
	if err != nil {
		return err
	}
	return dm.setCurrentVolume(volumeName)
}

func (dm *DatameshAPI) setCurrentVolume(volumeName string) error {
	return dm.Configuration.SetCurrentVolume(volumeName)
}

func (dm *DatameshAPI) setCurrentBranch(volumeName, branchName string) error {
	// TODO make an API call here to switch running point for containers
	return dm.Configuration.SetCurrentBranchForVolume(volumeName, branchName)
}

func (dm *DatameshAPI) CreateBranch(volumeName, sourceBranch, newBranch string) error {
	var result bool

	namespace, name, err := ParseNamespacedVolume(volumeName)
	if err != nil {
		return err
	}

	commitId, err := dm.findCommit("HEAD", volumeName, sourceBranch)
	if err != nil {
		return err
	}

	return dm.client.CallRemote(
		context.Background(),
		"DatameshRPC.Clone",
		struct {
			// Create a named clone from a given volume+branch pair at a given
			// commit (that branch's latest commit)
			Namespace, Volume, SourceBranch, NewBranchName, SourceSnapshotId string
		}{
			Namespace:        namespace,
			Volume:           name,
			SourceBranch:     sourceBranch,
			SourceSnapshotId: commitId,
			NewBranchName:    newBranch,
		},
		&result,
	)
	/*
		TODO (maybe distinguish between `dm checkout -b` and `dm branch` based
		on whether or not we switch the active branch here)
		return dm.setCurrentBranch(volumeName, branchName)
	*/
}

func (dm *DatameshAPI) CheckoutBranch(volumeName, from, to string, create bool) error {
	namespace, name, err := ParseNamespacedVolume(volumeName)
	if err != nil {
		return err
	}

	exists, err := dm.BranchExists(volumeName, to)
	if err != nil {
		return err
	}
	// The DEFAULT_BRANCH always implicitly exists
	exists = exists || to == DEFAULT_BRANCH
	if create {
		if exists {
			return fmt.Errorf("Branch already exists: %s", to)
		}
		if err := dm.CreateBranch(volumeName, from, to); err != nil {
			return err
		}
	}
	if !create {
		if !exists {
			return fmt.Errorf("Branch does not exist: %s", to)
		}
	}
	if err := dm.setCurrentBranch(volumeName, to); err != nil {
		return err
	}
	var result bool
	err = dm.client.CallRemote(context.Background(),
		"DatameshRPC.SwitchContainers", map[string]string{
			"Namespace":              namespace,
			"TopLevelFilesystemName": name,
			"CurrentCloneName":       deMasterify(from),
			"NewCloneName":           deMasterify(to),
		}, &result)
	if err != nil {
		return err
	}
	return nil
}

func (dm *DatameshAPI) CurrentVolume() (string, error) {
	return dm.Configuration.CurrentVolume()
}

func (dm *DatameshAPI) BranchExists(volumeName, branchName string) (bool, error) {
	branches, err := dm.Branches(volumeName)
	if err != nil {
		return false, err
	}
	for _, branch := range branches {
		if branch == branchName {
			return true, nil
		}
	}
	return false, nil
}

func (dm *DatameshAPI) Branches(volumeName string) ([]string, error) {
	namespace, name, err := ParseNamespacedVolume(volumeName)
	if err != nil {
		return []string{}, err
	}

	branches := []string{}
	err = dm.client.CallRemote(
		context.Background(), "DatameshRPC.Clones", VolumeName{namespace, name}, &branches,
	)
	if err != nil {
		return []string{}, err
	}
	return branches, nil
}

func (dm *DatameshAPI) VolumeExists(volumeName string) (bool, error) {
	namespace, name, err := ParseNamespacedVolume(volumeName)
	if err != nil {
		return false, err
	}

	volumes := map[string]map[string]DatameshVolume{}
	err = dm.client.CallRemote(
		context.Background(), "DatameshRPC.List", nil, &volumes,
	)
	if err != nil {
		return false, err
	}

	volumesInNamespace, ok := volumes[namespace]
	if !ok {
		return false, nil
	}
	_, ok = volumesInNamespace[name]
	return ok, nil
}

func (dm *DatameshAPI) DeleteVolume(volumeName string) error {
	namespace, name, err := ParseNamespacedVolume(volumeName)
	if err != nil {
		return err
	}

	var result bool
	err = dm.client.CallRemote(
		context.Background(), "DatameshRPC.DeleteVolume", VolumeName{namespace, name}, &result,
	)
	if err != nil {
		return err
	}

	err = dm.Configuration.DeleteStateForVolume(volumeName)
	if err != nil {
		return err
	}
	return nil
}

func (dm *DatameshAPI) SwitchVolume(volumeName string) error {
	return dm.setCurrentVolume(volumeName)
}

func (dm *DatameshAPI) CurrentBranch(volumeName string) (string, error) {
	return dm.Configuration.CurrentBranchFor(volumeName)
}

func (dm *DatameshAPI) AllBranches(volumeName string) ([]string, error) {
	namespace, name, err := ParseNamespacedVolume(volumeName)
	if err != nil {
		return []string{}, err
	}

	var branches []string
	err = dm.client.CallRemote(
		context.Background(), "DatameshRPC.Clones", VolumeName{namespace, name}, &branches,
	)
	// the "main" filesystem (topLevelFilesystemId) is the master branch
	// (DEFAULT_BRANCH)
	branches = append(branches, DEFAULT_BRANCH)
	sort.Strings(branches)
	return branches, err
}

func (dm *DatameshAPI) AllVolumes() ([]DatameshVolume, error) {
	filesystems := map[string]map[string]DatameshVolume{}
	result := []DatameshVolume{}
	interim := map[string]DatameshVolume{}
	err := dm.client.CallRemote(
		context.Background(), "DatameshRPC.List", nil, &filesystems,
	)
	if err != nil {
		return result, err
	}
	names := []string{}
	for namespace, volumesInNamespace := range filesystems {
		var prefix string
		if namespace == "admin" {
			prefix = ""
		} else {
			prefix = namespace + "/"
		}

		for filesystem, v := range volumesInNamespace {
			name := prefix + filesystem
			interim[name] = v
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		result = append(result, interim[name])
	}
	return result, nil
}

func deMasterify(s string) string {
	// use empty string to indicate "no clone"
	if s == DEFAULT_BRANCH {
		return ""
	}
	return s
}

func (dm *DatameshAPI) Commit(activeVolumeName, activeBranch, commitMessage string) (string, error) {
	var result bool

	activeNamespace, activeVolume, err := ParseNamespacedVolume(activeVolumeName)
	if err != nil {
		return "", err
	}

	err = dm.client.CallRemote(
		context.Background(),
		"DatameshRPC.Snapshot",
		// TODO replace these map[string]string's with typed structs that are
		// shared between the client and the server for cross-rpc type safety
		map[string]string{
			"Namespace":              activeNamespace,
			"TopLevelFilesystemName": activeVolume,
			"CloneName":              deMasterify(activeBranch),
			"Message":                commitMessage,
		},
		&result,
	)
	if err != nil {
		return "", err
	}
	// TODO pass through the commit (snapshot) ID
	return "", nil
}

type metadata map[string]string
type snapshot struct {
	// exported for json serialization
	Id       string
	Metadata *metadata
}

func (dm *DatameshAPI) ListCommits(activeVolumeName, activeBranch string) ([]snapshot, error) {
	var result []snapshot

	activeNamespace, activeVolume, err := ParseNamespacedVolume(activeVolumeName)
	if err != nil {
		return []snapshot{}, err
	}

	err = dm.client.CallRemote(
		context.Background(),
		"DatameshRPC.Snapshots",
		map[string]string{
			"Namespace":              activeNamespace,
			"TopLevelFilesystemName": activeVolume,
			"CloneName":              deMasterify(activeBranch),
		},
		// TODO recusively prefix clones' origin snapshots (but error on
		// resetting to them, and maybe mark the origin snap in a particular
		// way in the 'dm log' output)
		&result,
	)
	if err != nil {
		return []snapshot{}, err
	}
	return result, nil
}

func (dm *DatameshAPI) findCommit(ref, volumeName, branchName string) (string, error) {
	hatRegex := regexp.MustCompile(`^HEAD\^*$`)
	if hatRegex.MatchString(ref) {
		countHats := len(ref) - len("HEAD")
		cs, err := dm.ListCommits(volumeName, branchName)
		if err != nil {
			return "", err
		}
		if len(cs) == 0 {
			return "", fmt.Errorf("No commits match %s", ref)
		}
		i := len(cs) - 1 - countHats
		if i < 0 {
			return "", fmt.Errorf("Commits don't go back that far")
		}
		return cs[i].Id, nil
	} else {
		return ref, nil
	}
}

func (dm *DatameshAPI) ResetCurrentVolume(commit string) error {
	activeVolume, err := dm.CurrentVolume()
	if err != nil {
		return err
	}
	namespace, name, err := ParseNamespacedVolume(activeVolume)
	if err != nil {
		return err
	}

	activeBranch, err := dm.CurrentBranch(activeVolume)
	if err != nil {
		return err
	}
	var result bool
	commitId, err := dm.findCommit(commit, activeVolume, activeBranch)
	if err != nil {
		return err
	}
	err = dm.client.CallRemote(
		context.Background(),
		"DatameshRPC.Rollback",
		map[string]string{
			"Namespace":              namespace,
			"TopLevelFilesystemName": name,
			"CloneName":              deMasterify(activeBranch),
			"SnapshotId":             commitId,
		},
		&result,
	)
	if err != nil {
		return err
	}
	return nil
}

type Container struct {
	Id   string
	Name string
}

func (dm *DatameshAPI) RelatedContainers(volumeName VolumeName, branch string) ([]Container, error) {
	result := []Container{}
	err := dm.client.CallRemote(
		context.Background(),
		"DatameshRPC.Containers",
		map[string]string{
			"Namespace":              volumeName.Namespace,
			"TopLevelFilesystemName": volumeName.Name,
			"CloneName":              deMasterify(branch),
		},
		&result,
	)
	if err != nil {
		return []Container{}, err
	}
	return result, nil
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

func (dm *DatameshAPI) PollTransfer(transferId string, out io.Writer) error {

	out.Write([]byte("Calculating...\n"))

	var bar *pb.ProgressBar
	started := false

	for {
		time.Sleep(time.Second)
		result := &TransferPollResult{}
		err := dm.client.CallRemote(
			context.Background(), "DatameshRPC.GetTransfer", transferId, result,
		)
		if err != nil {
			if !strings.Contains(fmt.Sprintf("%s", err), "No such intercluster transfer") {
				out.Write([]byte(fmt.Sprintf("Got error, trying again: %s\n", err)))
			}
		}
		if result.Size > 0 {
			if !started {
				bar = pb.New64(result.Size)
				bar.ShowFinalTime = false
				bar.SetMaxWidth(80)
				bar.SetUnits(pb.U_BYTES)
				bar.Start()
				started = true
			}
			// Numbers reported by data transferred thru datamesh versus size
			// of stream reported by 'zfs send -nP' are off by a few kilobytes,
			// fudge it (maybe no one will notice).
			if result.Sent > result.Size {
				bar.Set64(result.Size)
			} else {
				bar.Set64(result.Sent)
			}
			_ = fmt.Sprintf(
				"%s: transferred %.2f/%.2fMiB in %.2fs (%.2fMiB/s)...\n",
				result.Status,
				// bytes => mebibytes
				float64(result.Sent)/(1024*1024),
				float64(result.Size)/(1024*1024),
				// nanoseconds => seconds
				float64(result.NanosecondsElapsed)/(1000*1000*1000),
			)
			bar.Prefix(result.Status)
			speed := fmt.Sprintf(" %.2f MiB/s",
				// mib/sec
				(float64(result.Sent)/(1024*1024))/
					(float64(result.NanosecondsElapsed)/(1000*1000*1000)),
			)
			quotient := fmt.Sprintf(" (%d/%d)", result.Index, result.Total)
			bar.Postfix(speed + quotient)
		}
		if result.Index == result.Total && result.Status == "finished" {
			if started {
				bar.FinishPrint("Done!")
			}
			// A terrible hack: many of the tests race the next 'dm log' or
			// similar command against snapshots received by a push/pull/clone
			// updating etcd which updates nodes' local caches of state. Give
			// the etcd updates a 1 second head start, which might reduce the
			// incidence of test flakes.
			// TODO: In general, we need a better way to _request_ the current
			// state of snapshots on a node, rather than always consulting
			// potentially out-of-date global caches. This might help with
			// scaling, too.
			time.Sleep(time.Second)
			return nil
		}
		if result.Status == "error" {
			if started {
				bar.FinishPrint(fmt.Sprintf("error: %s", result.Message))
			}
			out.Write([]byte(result.Message + "\n"))
			// A similarly terrible hack. See comment above.
			time.Sleep(time.Second)
			return fmt.Errorf(result.Message)
		}
	}
}

/*

pull
----

  to   from
  O*<-----O

push
----

  from   to
  O*----->O

* = current

*/

type TransferRequest struct {
	Peer                 string
	User                 string
	ApiKey               string
	Direction            string
	LocalNamespace       string
	LocalFilesystemName  string
	LocalCloneName       string
	RemoteNamespace      string
	RemoteFilesystemName string
	RemoteCloneName      string
	TargetSnapshot       string
}

// attempt to get the latest commits in filesystemId (which may be a branch)
// from fromRemote to toRemote as a one-off.
//
// the reason for supporting both directions is that the "current" is often
// behind NAT from its peer, and so it must initiate the connection.
func (dm *DatameshAPI) RequestTransfer(
	direction, peer,
	localFilesystemName, localBranchName,
	remoteFilesystemName, remoteBranchName string,
) (string, error) {
	connectionInitiator := dm.Configuration.CurrentRemote

	/*
		fmt.Printf("[DEBUG RequestTransfer] dir=%s peer=%s lfs=%s lb=%s rfs=%s rb=%s\n",
			direction, peer,
			localFilesystemName, localBranchName,
			remoteFilesystemName, remoteBranchName)
	*/

	var err error

	remote, err := dm.Configuration.GetRemote(peer)
	if err != nil {
		return "", err
	}

	// Let's replace any missing things with defaults.
	// The defaults depend on whether we're pushing or pulling.

	if direction == "push" {
		// We are pushing, so if no local filesystem/branch is
		// specified, take the current one.
		if localFilesystemName == "" {
			localFilesystemName, err = dm.Configuration.CurrentVolume()
			if err != nil {
				return "", err
			}
		}

		if localBranchName == "" {
			localBranchName, err = dm.Configuration.CurrentBranch()
			if err != nil {
				return "", err
			}
		}
	} else if direction == "pull" {
		// We are pulling, so if no local filesystem/branch is
		// specified, we take the remote name but strip it of its
		// namespace. So if we pull "bob/apples", we pull into "apples",
		// which is really "admin/apples".
		if localFilesystemName == "" && remoteFilesystemName != "" {
			_, localFilesystemName, err = ParseNamespacedVolume(remoteFilesystemName)
			if err != nil {
				return "", err
			}
		}
	}

	// Split the local volume name's namespace out
	localNamespace, localVolume, err := ParseNamespacedVolume(localFilesystemName)
	if err != nil {
		return "", err
	}

	// Guess defaults for the remote filesystem
	var remoteNamespace, remoteVolume string

	if remoteFilesystemName == "" {
		// No remote specified. Do we already have a default configured?
		defaultRemoteNamespace, defaultRemoteVolume, ok := dm.Configuration.DefaultRemoteVolumeFor(peer, localNamespace, localVolume)
		if ok {
			// If so, use it
			remoteNamespace = defaultRemoteNamespace
			remoteVolume = defaultRemoteVolume
		} else {
			// If not, default to the un-namespaced local filesystem name.
			// This causes it to default into the user's own namespace
			// when we parse the name, too.
			remoteNamespace = remote.User
			remoteVolume = localVolume
		}
	} else {
		// Default namespace for remote volume is the username on this remote
		remoteNamespace, remoteVolume, err = ParseNamespacedVolumeWithDefault(remoteFilesystemName, remote.User)
		if err != nil {
			return "", err
		}
	}

	// Remember default remote if there isn't already one
	_, _, ok := dm.Configuration.DefaultRemoteVolumeFor(peer, localNamespace, localVolume)
	if !ok {
		dm.Configuration.SetDefaultRemoteVolumeFor(peer, localNamespace, localVolume, remoteNamespace, remoteVolume)
	}

	if remoteBranchName == "" {
		remoteBranchName = localBranchName
	}

	if remoteBranchName != "" && remoteVolume == "" {
		return "", fmt.Errorf(
			"It's dubious to specify a remote branch name " +
				"without specifying a remote filesystem name.",
		)
	}

	if direction == "push" {
		fmt.Printf("Pushing %s/%s to %s:%s/%s\n",
			localNamespace, localVolume,
			peer,
			remoteNamespace, remoteVolume,
		)
	} else {
		fmt.Printf("Pulling %s/%s from %s:%s/%s\n",
			localNamespace, localVolume,
			peer,
			remoteNamespace, remoteVolume,
		)
	}

	// connect to connectionInitiator
	client, err := dm.Configuration.ClusterFromRemote(connectionInitiator)
	if err != nil {
		return "", err
	}
	var transferId string
	// TODO make ApiKey time- and domain- (filesystem?) limited
	// cryptographically somehow
	err = client.CallRemote(context.Background(),
		"DatameshRPC.Transfer", TransferRequest{
			Peer:                 remote.Hostname,
			User:                 remote.User,
			ApiKey:               remote.ApiKey,
			Direction:            direction,
			LocalNamespace:       localNamespace,
			LocalFilesystemName:  localVolume,
			LocalCloneName:       deMasterify(localBranchName),
			RemoteNamespace:      remoteNamespace,
			RemoteFilesystemName: remoteVolume,
			RemoteCloneName:      deMasterify(remoteBranchName),
			// TODO add TargetSnapshot here, to support specifying "push to a given
			// snapshot" rather than just "push all snapshots up to the latest"
		}, &transferId)
	if err != nil {
		return "", err
	}
	return transferId, nil
}

// FIXME: Put this in a shared library, as it duplicates the copy in
// datamesh-server/pkg/main/utils.go (now with a few differences)

type VolumeName struct {
	Namespace string
	Name      string
}

func (v VolumeName) String() string {
	if v.Namespace == "admin" {
		return v.Name
	} else {
		return fmt.Sprintf("%s/%s", v.Namespace, v.Name)
	}
}

func ParseNamespacedVolumeWithDefault(name, defaultNamespace string) (string, string, error) {
	parts := strings.Split(name, "/")
	switch len(parts) {
	case 0: // name was empty
		return "", "", nil
	case 1: // name was unqualified, no namespace, so we default
		return defaultNamespace, name, nil
	case 2: // Qualified name
		return parts[0], parts[1], nil
	default: // Too many slashes!
		return "", "", fmt.Errorf("Volume names must be of the form NAMESPACE/VOLUME or just VOLUME: '%s'", name)
	}
}

func ParseNamespacedVolume(name string) (string, string, error) {
	return ParseNamespacedVolumeWithDefault(name, "admin")
}
