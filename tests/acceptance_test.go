package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

/*

Take a look at docs/dev-commands.md to see how to run these tests.

*/

func TestSingleNode(t *testing.T) {
	// single node tests
	teardownFinishedTestRuns()

	f := Federation{NewCluster(1)}

	startTiming()
	err := f.Start(t)
	defer testMarkForCleanup(f)
	if err != nil {
		t.Error(err)
	}
	node1 := f[0].GetNode(0).Container

	// Sub-tests, to reuse common setup code.
	t.Run("Commit", func(t *testing.T) {
		fsname := uniqName()
		d(t, node1, dockerRun(fsname)+" touch /foo/X")
		d(t, node1, "dm switch "+fsname)
		d(t, node1, "dm commit -m 'hello'")
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message in log output")
		}
	})

	t.Run("Branch", func(t *testing.T) {
		fsname := uniqName()
		d(t, node1, dockerRun(fsname)+" touch /foo/X")
		d(t, node1, "dm switch "+fsname)
		d(t, node1, "dm commit -m 'hello'")
		d(t, node1, "dm checkout -b branch1")
		d(t, node1, dockerRun(fsname)+" touch /foo/Y")
		d(t, node1, "dm commit -m 'there'")
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "there") {
			t.Error("unable to find commit message in log output")
		}
		d(t, node1, "dm checkout master")
		resp = s(t, node1, dockerRun(fsname)+" ls /foo/")
		if strings.Contains(resp, "Y") {
			t.Error("failed to switch filesystem")
		}
		d(t, node1, "dm checkout branch1")
		resp = s(t, node1, dockerRun(fsname)+" ls /foo/")
		if !strings.Contains(resp, "Y") {
			t.Error("failed to switch filesystem")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		fsname := uniqName()
		d(t, node1, dockerRun(fsname)+" touch /foo/X")
		d(t, node1, "dm switch "+fsname)
		d(t, node1, "dm commit -m 'hello'")
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message in log output")
		}
		d(t, node1, dockerRun(fsname)+" touch /foo/Y")
		d(t, node1, "dm commit -m 'again'")
		resp = s(t, node1, "dm log")
		if !strings.Contains(resp, "again") {
			t.Error("unable to find commit message in log output")
		}
		d(t, node1, "dm reset --hard HEAD^")
		resp = s(t, node1, "dm log")
		if strings.Contains(resp, "again") {
			t.Error("found 'again' in dm log when i shouldn't have")
		}
		// check filesystem got rolled back
		resp = s(t, node1, dockerRun(fsname)+" ls /foo/")
		if strings.Contains(resp, "Y") {
			t.Error("failed to roll back filesystem")
		}
	})

	t.Run("RunningContainersListed", func(t *testing.T) {
		fsname := uniqName()
		d(t, node1, dockerRun(fsname, "-d --name tester")+" sleep 100")
		resp := s(t, node1, "dm list")
		if !strings.Contains(resp, "tester") {
			t.Error("container running not listed")
		}
	})

	// TODO test AllVolumesAndClones
	t.Run("AllVolumesAndClones", func(t *testing.T) {
		resp := s(t, node1, "dm debug AllVolumesAndClones")
		fmt.Printf("AllVolumesAndClones response: %v\n", resp)
	})

}

func TestTwoNodesSameCluster(t *testing.T) {
	teardownFinishedTestRuns()

	f := Federation{NewCluster(2)}

	startTiming()
	err := f.Start(t)
	defer testMarkForCleanup(f)
	if err != nil {
		t.Error(err)
	}
	logTiming("setup")

	node1 := f[0].GetNode(0).Container
	node2 := f[0].GetNode(1).Container

	t.Run("Move", func(t *testing.T) {
		fsname := uniqName()
		d(t, node1, dockerRun(fsname)+" sh -c 'echo WORLD > /foo/HELLO'")
		st := s(t, node2, dockerRun(fsname)+" cat /foo/HELLO")
		if !strings.Contains(st, "WORLD") {
			t.Error(fmt.Sprintf("Unable to find world in transported data capsule, got '%s'", st))
		}
	})
}

func TestTwoSingleNodeClusters(t *testing.T) {
	teardownFinishedTestRuns()

	f := Federation{
		NewCluster(1), // cluster_0_node_0
		NewCluster(1), // cluster_1_node_0
	}
	startTiming()
	err := f.Start(t)
	defer testMarkForCleanup(f)
	if err != nil {
		t.Error(err)
	}
	node1 := f[0].GetNode(0).Container
	node2 := f[1].GetNode(0).Container

	t.Run("PushCommitBranchExtantBase", func(t *testing.T) {
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push cluster_0")

		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message remote's log output")
		}
		// test incremental push
		d(t, node2, "dm commit -m 'again'")
		d(t, node2, "dm push cluster_0")

		resp = s(t, node1, "dm log")
		if !strings.Contains(resp, "again") {
			t.Error("unable to find commit message remote's log output")
		}
		// test pushing branch with extant base
		d(t, node2, "dm checkout -b newbranch")
		d(t, node2, "dm commit -m 'branchy'")
		d(t, node2, "dm push cluster_0")

		d(t, node1, "dm checkout newbranch")
		resp = s(t, node1, "dm log")
		if !strings.Contains(resp, "branchy") {
			t.Error("unable to find commit message remote's log output")
		}
	})
	t.Run("PushCommitBranchNoExtantBase", func(t *testing.T) {
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		// test pushing branch with no base on remote
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'master'")
		d(t, node2, "dm checkout -b newbranch")
		d(t, node2, "dm commit -m 'branchy'")
		d(t, node2, "dm checkout -b newbranch2")
		d(t, node2, "dm commit -m 'branchy2'")
		d(t, node2, "dm checkout -b newbranch3")
		d(t, node2, "dm commit -m 'branchy3'")
		d(t, node2, "dm push cluster_0")

		d(t, node1, "dm switch "+fsname)
		d(t, node1, "dm checkout newbranch3")
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "branchy3") {
			t.Error("unable to find commit message remote's log output")
		}
	})
	t.Run("DirtyDetected", func(t *testing.T) {
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push cluster_0")

		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message remote's log output")
		}
		// now dirty the filesystem on node1 w/1MB before it can be received into
		d(t, node1, dockerRun(""+fsname+"")+" dd if=/dev/urandom of=/foo/Y bs=1024 count=1024")

		for i := 0; i < 10; i++ {
			dirty, err := strconv.Atoi(strings.TrimSpace(
				s(t, node1, "dm list -H |grep "+fsname+" |cut -f 7"),
			))
			if err != nil {
				t.Error(err)
			}
			if dirty > 0 {
				break
			}
			fmt.Printf("Not dirty yet, waiting...\n")
			time.Sleep(time.Duration(i) * time.Second)
		}

		// test incremental push
		d(t, node2, "dm commit -m 'again'")
		result := s(t, node2, "dm push cluster_0 || true") // an error code is ok

		if !strings.Contains(result, "uncommitted") {
			t.Error("pushing didn't fail when there were known uncommited changes on the peer")
		}
	})
	t.Run("DirtyImmediate", func(t *testing.T) {
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push cluster_0")

		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message remote's log output")
		}
		// now dirty the filesystem on node1 w/1MB before it can be received into
		d(t, node1, dockerRun(""+fsname+"")+" dd if=/dev/urandom of=/foo/Y bs=1024 count=1024")

		// test incremental push
		d(t, node2, "dm commit -m 'again'")
		result := s(t, node2, "dm push cluster_0 || true") // an error code is ok

		if !strings.Contains(result, "has been modified") {
			t.Error(
				"pushing didn't fail when there were known uncommited changes on the peer",
			)
		}
	})
	t.Run("Diverged", func(t *testing.T) {
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push cluster_0")

		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message remote's log output")
		}
		// now make a commit that will diverge the filesystems
		d(t, node1, "dm commit -m 'node1 commit'")

		// test incremental push
		d(t, node2, "dm commit -m 'node2 commit'")
		result := s(t, node2, "dm push cluster_0 || true") // an error code is ok

		if !strings.Contains(result, "diverged") && !strings.Contains(result, "hello") {
			t.Error(
				"pushing didn't fail when there was a divergence",
			)
		}
	})
	t.Run("ResetAfterPushThenPushMySQL", func(t *testing.T) {
		fsname := uniqName()
		d(t, node2, dockerRun(
			fsname, "-d -e MYSQL_ROOT_PASSWORD=secret", "mysql:5.7.17", "/var/lib/mysql",
		))
		time.Sleep(10 * time.Second)
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push cluster_0")

		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message remote's log output")
		}
		// now make a commit that will diverge the filesystems
		d(t, node1, "dm commit -m 'node1 commit'")

		// test resetting a commit made on a pushed volume
		d(t, node2, "dm commit -m 'node2 commit'")
		d(t, node1, "dm reset --hard HEAD^")
		resp = s(t, node1, "dm log")
		if strings.Contains(resp, "node1 commit") {
			t.Error("found 'node1 commit' in dm log when i shouldn't have")
		}
		d(t, node2, "dm push cluster_0")
		resp = s(t, node1, "dm log")
		if !strings.Contains(resp, "node2 commit") {
			t.Error("'node2 commit' didn't make it over to node1 after reset-and-push")
		}
	})
	t.Run("PushToAuthorizedUser", func(t *testing.T) {
		// TODO
		// create a user on the second cluster. on the first cluster, push a
		// volume that user's account.
	})
	t.Run("NoPushToUnauthorizedUser", func(t *testing.T) {
		// TODO
		// a user can't push to a volume they're not authorized to push to.
	})
	t.Run("PushToCollaboratorVolume", func(t *testing.T) {
		// TODO
		// after adding another user as a collaborator, it's possible to push
		// to their volume.
	})
	t.Run("Clone", func(t *testing.T) {
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		// XXX 'dm clone' currently tries to pull the named filesystem into the
		// _current active filesystem name_. instead, it should pull it into a
		// new filesystem with the same name. if the same named filesystem
		// already exists, it should error (and instruct the user to 'dm switch
		// foo; dm pull foo' instead).
		d(t, node1, "dm clone cluster_1 "+fsname)
		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			// TODO fix this failure by sending prelude in intercluster case also
			t.Error("unable to find commit message remote's log output")
		}
		// test incremental pull
		d(t, node2, "dm commit -m 'again'")
		d(t, node1, "dm pull cluster_1 "+fsname)

		resp = s(t, node1, "dm log")
		if !strings.Contains(resp, "again") {
			t.Error("unable to find commit message remote's log output")
		}
		// test pulling branch with extant base
		d(t, node2, "dm checkout -b newbranch")
		d(t, node2, "dm commit -m 'branchy'")
		d(t, node1, "dm pull cluster_1 "+fsname+" newbranch")

		d(t, node1, "dm checkout newbranch")
		resp = s(t, node1, "dm log")
		if !strings.Contains(resp, "branchy") {
			t.Error("unable to find commit message remote's log output")
		}
	})
}

func TestFrontend(t *testing.T) {
	teardownFinishedTestRuns()

	f := Federation{NewCluster(1)}

	userLogin := uniqLogin()

	startTiming()
	err := f.Start(t)
	defer testMarkForCleanup(f)
	if err != nil {
		t.Error(err)
	}
	node1 := f[0].GetNode(0).Container

	t.Run("Authenticate", func(t *testing.T) {

		// start chrome driver
		startChromeDriver(t, node1)
		defer stopChromeDriver(t, node1)

		runFrontendTest(t, node1, "specs/auth.js", userLogin)

		// create the account locally so we can 'dm' with the same user that we
		// register with.  need to do this AFTER we have register because dm
		// remote add will check the api server with deets.
		d(t, node1,
			fmt.Sprintf(
				"DATAMESH_PASSWORD=%s dm remote add testremote %s@localhost",
				userLogin.Password, userLogin.Username,
			),
		)

		d(t, node1, "dm remote switch local")
		d(t, node1, "dm init testvolume")
		d(t, node1, "dm list")

		runFrontendTest(t, node1, "specs/volumes.js", userLogin)
		runFrontendTest(t, node1, "specs/rememberme.js", userLogin)

		copyMedia(node1)
	})
}

func TestKubernetes(t *testing.T) {
	teardownFinishedTestRuns()

	f := Federation{NewKubernetes(3)}

	startTiming()
	err := f.Start(t)
	defer testMarkForCleanup(f)
	if err != nil {
		t.Error(err)
	}
	node1 := f[0].GetNode(0).Container

	t.Run("ListEmpty", func(t *testing.T) {
		// TODO: maybe need to grab the password created by NewKubernetes and
		// write it to dm config
		//
		// dm list should succeed in connecting to the datamesh cluster
		d(t, node1, "dm list")
	})
}
