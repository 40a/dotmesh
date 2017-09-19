package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"io"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/coreos/etcd/client"
)

// A branch is just another filesystem, but which exists as a ZFS clone of a
// snapshot of another (filesystem or clone).
//
// The Registry allows us to record both top level filesystem name => id
// mappings, as well as knowledge about clones and their origins (the
// filesystem id and snapshot from which they were cloned).

const DEFAULT_BRANCH = "master"

type Registry struct {
	// filesystems ~= repos, top-level filesystems
	// map user facing filesystem name => filesystemId, with implicit null
	// origin
	TopLevelFilesystems     map[string]TopLevelFilesystem
	TopLevelFilesystemsLock sync.Mutex
	// clones ~= branches
	// map filesystem.id (of topLevelFilesystem the clone is attributed to - ie
	// not another clone) => user facing *branch name* => filesystemId,origin pair
	Clones     map[string]map[string]Clone
	ClonesLock sync.Mutex
	state      *InMemoryState
}

func (r *Registry) deducePathToTopLevelFilesystem(topLevelFilesystemName, cloneName string) (
	PathToTopLevelFilesystem, error,
) {
	/*
		Need to give the peer enough information to recreate an entire path from
		root to leaf of clone metadata. Example:

			master
			|- branch1
			\- branch2
		       \- branch2b

		If this filesystem id represents branch2b, the response would be
		[]string{"master", "branch2", "branch2b"}

		Except, it actually needs to be []Clone{...} with each clone referring
		to its origin, so that the appropriate data can be reproduced in the
		peer's registry.

	*/
	log.Printf("[deducePathToTopLevelFilesystem] looking up %s", topLevelFilesystemName)
	tlf, err := r.LookupFilesystem(topLevelFilesystemName)
	if err != nil {
		log.Printf(
			"[deducePathToTopLevelFilesystem] error looking up %s: %s",
			topLevelFilesystemName, err,
		)
		return PathToTopLevelFilesystem{}, err
	}
	log.Printf(
		"[deducePathToTopLevelFilesystem] looking up maybe-clone pair %s,%s",
		topLevelFilesystemName, cloneName,
	)
	filesystemId, err := r.MaybeCloneFilesystemId(topLevelFilesystemName, cloneName)
	if err != nil {
		log.Printf(
			"[deducePathToTopLevelFilesystem] error looking up maybe-clone %s,%s: %s",
			topLevelFilesystemName, cloneName, err,
		)
		return PathToTopLevelFilesystem{}, err
	}
	nextFilesystemId := filesystemId

	clist := ClonesList{}

	for {
		log.Printf(
			"[deducePathToTopLevelFilesystem] %s == %s ?",
			nextFilesystemId, tlf.TopLevelVolume.Id,
		)
		// base case - nextFilesystemId is the top level one.
		if nextFilesystemId == tlf.TopLevelVolume.Id {
			return PathToTopLevelFilesystem{
				TopLevelFilesystemId:   nextFilesystemId,
				TopLevelFilesystemName: topLevelFilesystemName,
				Clones:                 clist, // empty on first iteration
			}, nil
		}
		// inductive step - resolve nextFilesystemId into its clone, if it is a
		// clone. if it's not a clone, and it's not a top level filesystem,
		// throw an error.
		clone, cloneName, err := r.LookupCloneByIdWithName(nextFilesystemId)
		if err != nil {
			return PathToTopLevelFilesystem{}, err
		}
		// append to beginning of list, because they need to be created in the
		// reverse order of traversal. (traversal is from tip to root, we want
		// to return the list from the root to tip.)
		clist = append(ClonesList{CloneWithName{Name: cloneName, Clone: clone}}, clist...)
		nextFilesystemId = clone.Origin.FilesystemId
	}
}

func NewRegistry(s *InMemoryState) *Registry {
	return &Registry{
		state:               s,
		TopLevelFilesystems: map[string]TopLevelFilesystem{},
		Clones:              map[string]map[string]Clone{},
	}
}

// sorted list of top-level filesystem names
func (r *Registry) Filesystems() []string {
	r.TopLevelFilesystemsLock.Lock()
	defer r.TopLevelFilesystemsLock.Unlock()
	filesystemNames := []string{}
	for name, _ := range r.TopLevelFilesystems {
		filesystemNames = append(filesystemNames, name)
	}
	sort.Strings(filesystemNames)
	return filesystemNames
}

func (r *Registry) IdFromName(name string) (string, error) {
	tlf, err := r.GetByName(name)
	if err != nil {
		return "", err
	}
	return tlf.TopLevelVolume.Id, nil
}

func (r *Registry) GetByName(name string) (TopLevelFilesystem, error) {
	r.TopLevelFilesystemsLock.Lock()
	defer r.TopLevelFilesystemsLock.Unlock()
	tlf, ok := r.TopLevelFilesystems[name]
	if !ok {
		return TopLevelFilesystem{},
			fmt.Errorf("No such top-level filesystem")
	}
	return tlf, nil
}

// list of filesystem ids
func (r *Registry) FilesystemIds() []string {
	r.TopLevelFilesystemsLock.Lock()
	defer r.TopLevelFilesystemsLock.Unlock()
	filesystemIds := []string{}
	for _, tlf := range r.TopLevelFilesystems {
		filesystemIds = append(filesystemIds, tlf.TopLevelVolume.Id)
	}
	sort.Strings(filesystemIds)
	return filesystemIds
}

// map of clone names => clone objects for a given top-level filesystemId
func (r *Registry) ClonesFor(filesystemId string) map[string]Clone {
	r.ClonesLock.Lock()
	defer r.ClonesLock.Unlock()
	_, ok := r.Clones[filesystemId]
	if !ok {
		// filesystemId not found, return empty map
		return map[string]Clone{}
	}
	return r.Clones[filesystemId]
}

// Check whether a given clone can be pulled onto this machine, based on
// whether its origin snapshot exists here
func (r *Registry) CanPullClone(c Clone) bool {
	r.state.filesystemsLock.Lock()
	fsMachine, ok := (*r.state.filesystems)[c.Origin.FilesystemId]
	r.state.filesystemsLock.Unlock()
	if !ok {
		return false
	}
	fsMachine.snapshotsLock.Lock()
	defer fsMachine.snapshotsLock.Lock()
	for _, snap := range fsMachine.filesystem.snapshots {
		if snap.Id == c.Origin.SnapshotId {
			return true
		}
	}
	return false
}

// the type as stored in the json in etcd (intermediate representation wrt
// DatameshVolume)
type registryFilesystem struct {
	Id              string
	OwnerId         string
	CollaboratorIds []string
}

// update a filesystem, including updating etcd and our local state
func (r *Registry) RegisterFilesystem(ctx context.Context, name string, filesystemId string) error {
	kapi, err := getEtcdKeysApi()
	if err != nil {
		return err
	}
	authenticatedUserId := ctx.Value("authenticated-user-id").(string)
	if authenticatedUserId == "" {
		return fmt.Errorf("No user found in request context.")
	}
	rf := registryFilesystem{
		Id: filesystemId,
		// Owner is, for now, always the authenticated user at the time of
		// creation
		OwnerId: authenticatedUserId,
	}
	serialized, err := json.Marshal(rf)
	if err != nil {
		return err
	}
	_, err = kapi.Set(
		context.Background(),
		// (0)/(1)datamesh.io/(2)registry/(3)filesystems/(4)<name> =>
		//     {"Uuid": "<fs-uuid>"}
		fmt.Sprintf("%s/registry/filesystems/%s", ETCD_PREFIX, name),
		string(serialized),
		// we support updates in UpdateCollaborators, below.
		&client.SetOptions{PrevExist: client.PrevNoExist},
	)
	if err != nil {
		return err
	}
	// Only update our local belief system once the write to etcd has been
	// successful!
	return r.UpdateFilesystemFromEtcd(name, rf)
}

func (r *Registry) UpdateCollaborators(
	ctx context.Context, tlf TopLevelFilesystem, newCollaborators []SafeUser,
) error {
	collaboratorIds := []string{}
	for _, u := range newCollaborators {
		collaboratorIds = append(collaboratorIds, u.Id)
	}
	rf := registryFilesystem{
		Id: tlf.TopLevelVolume.Id,
		// Owner is, for now, always the authenticated user at the time of
		// creation
		OwnerId:         tlf.Owner.Id,
		CollaboratorIds: collaboratorIds,
	}
	serialized, err := json.Marshal(rf)
	if err != nil {
		return err
	}
	kapi, err := getEtcdKeysApi()
	if err != nil {
		return err
	}
	_, err = kapi.Set(
		context.Background(),
		// (0)/(1)datamesh.io/(2)registry/(3)filesystems/(4)<name> =>
		//     {"Uuid": "<fs-uuid>"}
		fmt.Sprintf("%s/registry/filesystems/%s", ETCD_PREFIX, tlf.TopLevelVolume.Name),
		string(serialized),
		// allow (and require) update over existing.
		&client.SetOptions{PrevExist: client.PrevExist},
	)
	if err != nil {
		return err
	}
	// Only update our local belief system once the write to etcd has been
	// successful!
	return r.UpdateFilesystemFromEtcd(tlf.TopLevelVolume.Name, rf)
}

// update a clone, including updating our local record and etcd
func (r *Registry) RegisterClone(name string, topLevelFilesystemId string, clone Clone) error {
	r.UpdateCloneFromEtcd(name, topLevelFilesystemId, clone)
	kapi, err := getEtcdKeysApi()
	if err != nil {
		return err
	}
	serialized, err := json.Marshal(clone)
	if err != nil {
		return err
	}
	kapi.Set(
		context.Background(),
		// (0)/(1)datamesh.io/(2)registry/(3)clones/(4)<fs-uuid-of-filesystem>/(5)<name> =>
		//     {"Origin": {"FilesystemId": "<fs-uuid-of-actual-origin-snapshot>", "SnapshotId": "<snap-id>"}, "Uuid": "<fs-uuid>"}
		fmt.Sprintf("%s/registry/clones/%s/%s", ETCD_PREFIX, topLevelFilesystemId, name),
		string(serialized),
		&client.SetOptions{PrevExist: client.PrevNoExist},
	)
	return nil
}

func safeUser(u User) SafeUser {
	h := md5.New()
	io.WriteString(h, u.Email)
	emailHash := fmt.Sprintf("%x", h.Sum(nil))
	return SafeUser{
		Id:          u.Id,
		Name:        u.Name,
		Email:       u.Email,
		EmailHash:   emailHash,
		CustomerId:  u.CustomerId,
		CurrentPlan: u.CurrentPlan,
	}
}

func (r *Registry) UpdateFilesystemFromEtcd(
	name string, rf registryFilesystem,
) error {
	r.TopLevelFilesystemsLock.Lock()
	defer r.TopLevelFilesystemsLock.Unlock()

	us, err := AllUsers()
	if err != nil {
		return err
	}
	umap := map[string]User{}
	for _, u := range us {
		umap[u.Id] = u
	}

	owner, ok := umap[rf.OwnerId]
	if !ok {
		return fmt.Errorf("Unable to locate owner %v.", rf.OwnerId)
	}

	collaborators := []SafeUser{}
	for _, c := range rf.CollaboratorIds {
		user, ok := umap[c]
		if !ok {
			return fmt.Errorf("Unable to locate collaborator.")
		}
		collaborators = append(collaborators, safeUser(user))
	}

	log.Printf("[UpdateFilesystemFromEtcd] %s => %s", name, rf.Id)
	r.TopLevelFilesystems[name] = TopLevelFilesystem{
		// XXX: Hmm, I wonder if it's OK to just put minimal information here.
		// Probably not! We should construct a real TopLevelFilesystem object
		// if that's even the right level of abstraction. At time of writing,
		// the only thing that seems to reasonably construct a
		// TopLevelFilesystem is rpc's AllVolumesAndClones.
		TopLevelVolume: DatameshVolume{Id: rf.Id, Name: name},
		Owner:          safeUser(owner),
		Collaborators:  collaborators,
	}
	return nil
}

func (r *Registry) UpdateCloneFromEtcd(name string, topLevelFilesystemId string, clone Clone) {
	r.ClonesLock.Lock()
	defer r.ClonesLock.Unlock()
	if _, ok := r.Clones[topLevelFilesystemId]; !ok {
		r.Clones[topLevelFilesystemId] = map[string]Clone{}
	}
	r.Clones[topLevelFilesystemId][name] = clone
}

func (r *Registry) LookupFilesystem(name string) (TopLevelFilesystem, error) {
	r.TopLevelFilesystemsLock.Lock()
	defer r.TopLevelFilesystemsLock.Unlock()
	if _, ok := r.TopLevelFilesystems[name]; !ok {
		return TopLevelFilesystem{}, fmt.Errorf("No such filesystem named '%s'", name)
	}
	return r.TopLevelFilesystems[name], nil
}

// XXX naming here is a mess, wrt LookupFilesystem{Id,Name} :/
func (r *Registry) LookupFilesystemName(filesystemId string) (name string, err error) {
	r.TopLevelFilesystemsLock.Lock()
	defer r.TopLevelFilesystemsLock.Unlock()
	// TODO make a more efficient data structure
	for name, tlf := range r.TopLevelFilesystems {
		if tlf.TopLevelVolume.Id == filesystemId {
			return name, nil
		}
	}
	return "", fmt.Errorf("No such filesystem with id '%s'", filesystemId)
}

// Look up a clone. If you want to look up based on filesystem name and clone name, do:
// fsId := LookupFilesystem(fsName); cloneId := LookupClone(fsId, cloneName)
func (r *Registry) LookupClone(topLevelFilesystemId, cloneName string) (Clone, error) {
	r.ClonesLock.Lock()
	defer r.ClonesLock.Unlock()
	if _, ok := r.Clones[topLevelFilesystemId]; !ok {
		return Clone{}, fmt.Errorf("No top-level filesystem id '%s'", topLevelFilesystemId)
	}
	if _, ok := r.Clones[topLevelFilesystemId][cloneName]; !ok {
		return Clone{}, fmt.Errorf("No clone named '%s'", cloneName)
	}
	return r.Clones[topLevelFilesystemId][cloneName], nil
}

type NoSuchClone struct {
	filesystemId string
}

func (n NoSuchClone) Error() string {
	return fmt.Sprintf("No clone with filesystem id '%s'", n.filesystemId)
}

// XXX make this more efficient
func (r *Registry) LookupCloneById(filesystemId string) (Clone, error) {
	c, _, err := r.LookupCloneByIdWithName(filesystemId)
	return c, err
}

func (r *Registry) LookupCloneByIdWithName(filesystemId string) (Clone, string, error) {
	r.ClonesLock.Lock()
	defer r.ClonesLock.Unlock()
	for _, cloneMap := range r.Clones {
		for cloneName, clone := range cloneMap {
			if clone.FilesystemId == filesystemId {
				return clone, cloneName, nil
			}
		}
	}
	return Clone{}, "", NoSuchClone{filesystemId}
}

// given a filesystem id, return the (topLevelFilesystem, cloneName) tuple that it
// can be identified by to the user.
// XXX make this less horrifically inefficient by storing & updating inverted
// indexes.
func (r *Registry) LookupFilesystemId(filesystemId string) (TopLevelFilesystem, string, error) {
	r.ClonesLock.Lock()
	defer r.ClonesLock.Unlock()
	for _, tlf := range r.TopLevelFilesystems {
		if tlf.TopLevelVolume.Id == filesystemId {
			// empty-string cloneName ~= "master branch"
			return tlf, "", nil
		}
	}
	for topLevelFilesystemId, cloneMap := range r.Clones {
		for cloneName, clone := range cloneMap {
			if clone.FilesystemId == filesystemId {
				// find the tlf for this topLevelFilesystemId
				for _, tlf := range r.TopLevelFilesystems {
					if tlf.TopLevelVolume.Id == topLevelFilesystemId {
						return tlf, cloneName, nil
					}
				}
			}
		}
	}
	return TopLevelFilesystem{}, "", fmt.Errorf(
		"Unable to find user-facing filesystemName, cloneName for filesystem id %s",
		filesystemId,
	)
}

// filesystem id if exists, else ""
func (r *Registry) Exists(topLevelFilesystemName, cloneName string) string {
	r.TopLevelFilesystemsLock.Lock()
	defer r.TopLevelFilesystemsLock.Unlock()
	tlf, ok := r.TopLevelFilesystems[topLevelFilesystemName]
	if !ok {
		return ""
	}
	filesystemId := tlf.TopLevelVolume.Id
	if cloneName != "" {
		r.ClonesLock.Lock()
		defer r.ClonesLock.Unlock()
		if _, ok := r.Clones[filesystemId]; !ok {
			return ""
		}
		clone, ok := r.Clones[filesystemId][cloneName]
		if !ok {
			return ""
		}
		filesystemId = clone.FilesystemId
	}
	return filesystemId
}

// given a top level fs name and a clone name, find the appropriate fs id
func (r *Registry) MaybeCloneFilesystemId(topLevelFilesystemName, cloneName string) (string, error) {
	tlf, err := r.LookupFilesystem(
		topLevelFilesystemName,
	)
	if err != nil {
		return "", err
	}
	tlfId := tlf.TopLevelVolume.Id
	if cloneName != "" {
		// potentially resolve a clone's filesystem id, clobbering filesystemId
		clone, err := r.LookupClone(tlfId, cloneName)
		if err != nil {
			return "", err
		}
		tlfId = clone.FilesystemId
	}
	return tlfId, nil
}
