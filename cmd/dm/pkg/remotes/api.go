package remotes

import (
	"fmt"
	"io"
	"regexp"
	"sort"
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
	Name        string
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
	err := dm.client.CallRemote(context.Background(), "DatameshRPC.Create", volumeName, &response)
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
			Volume, SourceBranch, NewBranchName, SourceSnapshotId string
		}{
			Volume:           volumeName,
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

func (dm *DatameshAPI) CheckoutBranch(volume, from, to string, create bool) error {
	exists, err := dm.BranchExists(volume, to)
	if err != nil {
		return err
	}
	// The DEFAULT_BRANCH always implicitly exists
	exists = exists || to == DEFAULT_BRANCH
	if create {
		if exists {
			return fmt.Errorf("Branch already exists: %s", to)
		}
		if err := dm.CreateBranch(volume, from, to); err != nil {
			return err
		}
	}
	if !create {
		if !exists {
			return fmt.Errorf("Branch does not exist: %s", to)
		}
	}
	if err := dm.setCurrentBranch(volume, to); err != nil {
		return err
	}
	var result bool
	err = dm.client.CallRemote(context.Background(),
		"DatameshRPC.SwitchContainers", map[string]string{
			"TopLevelFilesystemName": volume,
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
	branches := []string{}
	err := dm.client.CallRemote(
		context.Background(), "DatameshRPC.Clones", volumeName, &branches,
	)
	if err != nil {
		return []string{}, err
	}
	return branches, nil
}

func (dm *DatameshAPI) VolumeExists(volumeName string) (bool, error) {
	volumes := map[string]DatameshVolume{}
	err := dm.client.CallRemote(
		context.Background(), "DatameshRPC.List", nil, &volumes,
	)
	if err != nil {
		return false, err
	}
	for volume, _ := range volumes {
		if volume == volumeName {
			return true, nil
		}
	}
	return false, nil
}

func (dm *DatameshAPI) SwitchVolume(volumeName string) error {
	return dm.setCurrentVolume(volumeName)
}

func (dm *DatameshAPI) CurrentBranch(volumeName string) (string, error) {
	return dm.Configuration.CurrentBranchFor(volumeName)
}

func (dm *DatameshAPI) AllBranches(volumeName string) ([]string, error) {
	var branches []string
	err := dm.client.CallRemote(
		context.Background(), "DatameshRPC.Clones", volumeName, &branches,
	)
	// the "main" filesystem (topLevelFilesystemId) is the master branch
	// (DEFAULT_BRANCH)
	branches = append(branches, DEFAULT_BRANCH)
	sort.Strings(branches)
	return branches, err
}

func (dm *DatameshAPI) AllVolumes() ([]DatameshVolume, error) {
	filesystems := map[string]DatameshVolume{}
	result := []DatameshVolume{}
	interim := map[string]DatameshVolume{}
	err := dm.client.CallRemote(
		context.Background(), "DatameshRPC.List", nil, &filesystems,
	)
	if err != nil {
		return result, err
	}
	names := []string{}
	for filesystem, v := range filesystems {
		interim[filesystem] = v
		names = append(names, filesystem)
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

func (dm *DatameshAPI) Commit(activeVolume, activeBranch, commitMessage string) (string, error) {
	var result bool
	err := dm.client.CallRemote(
		context.Background(),
		"DatameshRPC.Snapshot",
		// TODO replace these map[string]string's with typed structs that are
		// shared between the client and the server for cross-rpc type safety
		map[string]string{
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

func (dm *DatameshAPI) ListCommits(activeVolume, activeBranch string) ([]snapshot, error) {
	var result []snapshot
	err := dm.client.CallRemote(
		context.Background(),
		"DatameshRPC.Snapshots",
		map[string]string{
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
			"TopLevelFilesystemName": activeVolume,
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

func (dm *DatameshAPI) RelatedContainers(volumeName, branch string) ([]Container, error) {
	result := []Container{}
	err := dm.client.CallRemote(
		context.Background(),
		"DatameshRPC.Containers",
		map[string]string{
			"TopLevelFilesystemName": volumeName,
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
			out.Write([]byte(fmt.Sprintf("Got error, trying again: %s\n", err)))
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
			return nil
		}
		if result.Status == "error" {
			if started {
				bar.FinishPrint(fmt.Sprintf("error: %s", result.Message))
			}
			out.Write([]byte(result.Message + "\n"))
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
	LocalFilesystemName  string
	LocalCloneName       string
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

	var err error
	var currentVolume string

	// Cases:
	// push without --remote-volume - remoteFilesystemName = ""
	// push with --remote-volume - remoteFilesystemName = remote volume
	// clone/pull - remoteFilesystemname = the filesystem we're playing with which also is the local one as we can't rename as part of the pull/clone
	
	if localFilesystemName == "" {
		currentVolume, err = dm.Configuration.CurrentVolume()
		if err != nil {
			return "", err
		}
	} else {
		currentVolume = localFilesystemName
	}

	if remoteBranchName != "" && remoteFilesystemName == "" {
		return "", fmt.Errorf(
			"It's dubious to specify a remote branch name " +
				"without specifying a remote filesystem name.",
		)
	}
	var currentBranch string
	if localBranchName == "" {
		currentBranch, err = dm.Configuration.CurrentBranch()
		if err != nil {
			return "", err
		}
	} else {
		currentBranch = localBranchName
	}

	// connect to connectionInitiator
	client, err := dm.Configuration.ClusterFromRemote(connectionInitiator)
	if err != nil {
		return "", err
	}
	remote, err := dm.Configuration.GetRemote(peer)
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
			LocalFilesystemName:  currentVolume,
			LocalCloneName:       deMasterify(currentBranch),
			RemoteFilesystemName: remoteFilesystemName,
			RemoteCloneName:      deMasterify(remoteBranchName),
			// TODO add TargetSnapshot here, to support specifying "push to a given
			// snapshot" rather than just "push all snapshots up to the latest"
		}, &transferId)
	if err != nil {
		return "", err
	}
	return transferId, nil
}
