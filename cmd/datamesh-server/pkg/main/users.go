package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coreos/etcd/client"
	"github.com/nu7hatch/gouuid"
)

// special admin user with global privs
const ADMIN_USER_UUID = "00000000-0000-0000-0000-000000000000"

// Create a brand new user, with a new user id
func NewUser(name, email, apiKey string) (User, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return User{}, err
	}
	// TODO enforce username and email address uniqueness
	return User{Id: id.String(), Name: name, Email: email, ApiKey: apiKey}, nil
}

// Write the user object to etcd
func (u User) Save() error {
	kapi, err := getEtcdKeysApi()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(u)
	if err != nil {
		return err
	}
	_, err = kapi.Set(
		context.Background(),
		fmt.Sprintf("%s/users/%s", ETCD_PREFIX, u.Id),
		string(encoded),
		nil,
	)
	return err
}

func AllUsers() ([]User, error) {
	users := []User{}

	kapi, err := getEtcdKeysApi()
	if err != nil {
		return users, err
	}

	// TODO perhaps it would be better to store users in memory rather than
	// fetching the entire list every time.
	allUsers, err := kapi.Get(
		context.Background(),
		fmt.Sprintf("%s/users", ETCD_PREFIX),
		&client.GetOptions{Recursive: true},
	)

	for _, u := range allUsers.Node.Nodes {
		this := User{}
		err := json.Unmarshal([]byte(u.Value), &this)
		if err != nil {
			return users, err
		}
		users = append(users, this)
	}
	return users, nil
}

func GetUserByName(name string) (User, error) {
	// naive
	us, err := AllUsers()
	if err != nil {
		return User{}, err
	}
	for _, u := range us {
		if u.Name == name {
			return u, nil
		}
	}
	return User{}, fmt.Errorf("User name=%v not found", name)
}

func GetUserById(id string) (User, error) {
	// naive
	us, err := AllUsers()
	if err != nil {
		return User{}, err
	}
	for _, u := range us {
		if u.Id == id {
			return u, nil
		}
	}
	return User{}, fmt.Errorf("User id=%v not found", id)
}

func (t TopLevelFilesystem) AuthorizeOwner(ctx context.Context) (bool, error) {
	return t.authorize(ctx, false)
}

func (t TopLevelFilesystem) Authorize(ctx context.Context) (bool, error) {
	return t.authorize(ctx, true)
}

func (t TopLevelFilesystem) authorize(ctx context.Context, includeCollab bool) (bool, error) {
	authenticatedUserId := ctx.Value("authenticated-user-id").(string)
	if authenticatedUserId == "" {
		return false, fmt.Errorf("No user found in context.")
	}
	// admin user is always authorized (e.g. docker daemon). users and auth are
	// only really meaningful over the network for data synchronization, when a
	// datamesh cluster is being used like a hub.
	if authenticatedUserId == ADMIN_USER_UUID {
		return true, nil
	}
	user, err := GetUserById(authenticatedUserId)
	if err != nil {
		return false, err
	}
	if user.Id == t.Owner.Id {
		return true, nil
	}
	if includeCollab {
		for _, other := range t.Collaborators {
			if user.Id == other.Id {
				return true, nil
			}
		}
	}
	return false, nil
}

func UserIsNamespaceAdministrator(userId, namespace string) (bool, error) {
	// Admin gets to administer every namespace
	if userId == ADMIN_USER_UUID {
		return true, nil
	}

	// Otherwise, look up the user...
	user, err := GetUserById(userId)
	if err != nil {
		return false, err
	}

	// ...and see if their name matches the namespace name. In future,
	// this can be extended to cover more configurable rules.
	if user.Name == namespace {
		return true, nil
	} else {
		return false, nil
	}
}

func AuthenticatedUserIsNamespaceAdministrator(ctx context.Context, namespace string) (bool, error) {
	u := ctx.Value("authenticated-user-id").(string)
	if u == "" {
		return false, fmt.Errorf("No user found in context.")
	}

	a, err := UserIsNamespaceAdministrator(u, namespace)
	return a, err
}
