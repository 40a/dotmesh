package main

/*

Datamesh acceptance test suite. Run with "sudo -E `which go` test" from
datamesh/cmd/datamesh-server.

** Assumes Ubuntu 16.10 **

This acceptance test suite uses docker-in-docker, kubeadm style. It creates
docker containers which simulate entire computers, each running systemd, and
then uses 'dm cluster init', etc, to set up datamesh. It requires internet
access only for the small amounts of configuration data and PKI material stored
in the datamesh discovery service. After the initial setup and priming of
docker images, which takes quite some time, it should take ~60 seconds to spin
up a 2 node datamesh cluster to run a test.

You should put the following docker config in /etc/docker/daemon.json:

{
    "storage-driver": "overlay2",
    "insecure-registries": ["$(hostname).local:80"]
}

Replacing $(hostname) with your hostname, and then `systemctl restart docker`.

You need to be running a local registry, available as part of the
github.com/lukemarsden/datamesh-instrumentation pack, which requires
docker-compose (run up.sh with a password as the first argument).

Finally, you need to be running github.com/lukemarsden/discovery.data-mesh.io
on port 8087:

	git clone git@github.com:lukemarsden/discovery.data-mesh.io
	cd discovery.data-mesh.io
	./start-local.sh

You have to do some one-off setup and priming of docker images before these
tests will run:

	cd $GOPATH/src; mkdir -p github.com/lukemarsden; cd github.com/lukemarsden
	git clone git@github.com:lukemarsden/datamesh
	cd ~/
	git clone git@github.com:kubernetes/kubernetes
	cd kubernetes
	git clone git@github.com:lukemarsden/kubeadm-dind-cluster dind
	dind/dind-cluster.sh bare prime-images
	docker rm -f prime-images
	cd $GOPATH/src/github.com/lukemarsden/datamesh/cmd/datamesh-server
	./rebuild.sh
	docker build -t $(hostname).local:80/lukemarsden/datamesh-server:pushpull .
	docker push $(hostname).local:80/lukemarsden/datamesh-server:pushpull

	docker pull quay.io/coreos/etcd:v3.0.15
	docker tag quay.io/coreos/etcd:v3.0.15 $(hostname).local:80/coreos/etcd:v3.0.15
	docker push $(hostname).local:80/coreos/etcd:v3.0.15

	docker pull busybox
	docker tag busybox $(hostname).local:80/busybox
	docker push $(hostname).local:80/busybox

	docker pull mysql:5.7.17
	docker tag mysql:5.7.17 $(hostname).local:80/mysql:5.7.17
	docker push $(hostname).local:80/mysql:5.7.17

	cd ~/
	git clone git@github.com:lukemarsden/datamesh-instrumentation
	cd datamesh-instrumentation
	cd etcd-browser
	docker build -t $(hostname).local:80/lukemarsden/etcd-browser:v1 .
	docker push $(hostname).local:80/lukemarsden/etcd-browser:v1

Now install some deps (for tests only):

	go get github.com/tools/godep
	apt install zfsutils-linux

You can now run tests, like:

	./mark-cleanup.sh; ./rebuild.sh && ./test.sh -run TestTwoSingleNodeClusters

*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

var timings map[string]float64
var lastTiming int64

func startTiming() {
	lastTiming = time.Now().UnixNano()
	timings = make(map[string]float64)
}

func logTiming(tag string) {
	now := time.Now().UnixNano()
	timings[tag] = float64(now-lastTiming) / (1000 * 1000 * 1000)
	lastTiming = now
}

func dumpTiming() {
	fmt.Printf("=== TIMING ===\n")
	for tag, timing := range timings {
		fmt.Printf("%s => %.2f\n", tag, timing)
	}
	fmt.Printf("=== END TIMING ===\n")
	timings = map[string]float64{}
}

func system(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func silentSystem(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	return c.Run()
}

func testMarkForCleanup(n int, stamp int64) {
	for i := 1; i < n+1; i++ {
		node := fmt.Sprintf("node_%d_%d", stamp, i)
		err := system("bash", "-c", fmt.Sprintf(
			`docker exec -t %s bash -c 'touch /CLEAN_ME_UP'`, node,
		))
		if err != nil {
			fmt.Printf("Error marking %s for cleanup: %s, retrying...\n", node, err)
			time.Sleep(1 * time.Second)
			err := system("bash", "-c", fmt.Sprintf(
				`docker exec -t %s bash -c 'touch /CLEAN_ME_UP'`, node,
			))
			if err != nil {
				fmt.Printf("Error marking %s for cleanup: %s, giving up.\n", node, err)
			}
		}
	}
}

func testSetup(n int, stamp int64) error {
	// build client once so we can copy it in place later
	err := system("bash", "-c", `
		# Create a home for the test pools to live that can have the same path
		# both from ZFS's perspective and that of the inner container.
		# (Bind-mounts all the way down.)
		mkdir -p /datamesh-test-pools
		# tmpfs makes etcd not completely rinse your IOPS (which it does
		# otherwise); create if doesn't exist
		if [ $(mount |grep "/tmpfs " |wc -l) -eq 0 ]; then
			mkdir -p /tmpfs && mount -t tmpfs -o size=4g tmpfs /tmpfs
		fi
	`)
	if err != nil {
		return err
	}

	for i := 1; i < n+1; i++ {
		node := fmt.Sprintf("node_%d_%d", stamp, i)
		// XXX the following only works if overlay is working
		err := system("bash", "-c", fmt.Sprintf(`
			mkdir -p /datamesh-test-pools
			MOUNTPOINT=/datamesh-test-pools
			NODE=%s
			if [ $(mount |grep $MOUNTPOINT |wc -l) -eq 0 ]; then
				echo "Creating and bind-mounting shared $MOUNTPOINT"
				mkdir -p $MOUNTPOINT && \
				mount --bind $MOUNTPOINT $MOUNTPOINT && \
				mount --make-shared $MOUNTPOINT;
			fi
			(cd ~/kubernetes && \
			EXTRA_DOCKER_ARGS="-v /datamesh-test-pools:/datamesh-test-pools:rshared" \
				dind/dind-cluster.sh quick $NODE)
			sleep 1
			docker exec -t $NODE bash -c '
				sed -i "s/docker daemon/docker daemon \
					--insecure-registry '$(hostname)'.local:80/" \
					/etc/systemd/system/docker.service.d/20-overlay.conf
				# workaround https://github.com/docker/docker/issues/19625
				sed -i "s/MountFlags=slave//" \
					/lib/systemd/system/docker.service
				systemctl daemon-reload
				systemctl restart docker
			'
			docker cp ../binaries/Linux/dm $NODE:/usr/local/bin/dm
		`, node))
		if err != nil {
			return err
		}
		fmt.Printf("=== Started up %s\n", node)
	}
	return nil
}

func teardownFinishedTestRuns() {
	cs, err := exec.Command(
		"docker", "ps", "--filter", "name=^/node_.*$", "--format", "{{.Names}}",
	).Output()
	if err != nil {
		panic(err)
	}
	stamps := []int{}
	for _, line := range strings.Split(string(cs), "\n") {
		shrap := strings.Split(line, "_")
		if len(shrap) > 1 {
			// node_timestamp_counter
			stamp := shrap[1]
			i, err := strconv.Atoi(stamp)
			if err != nil {
				panic(err)
			}
			stamps = append(stamps, i)
		}
	}

	sort.Ints(stamps)
	for _, stamp := range stamps {
		func() {
			maxNodesInAnyTest := 2 // XXX Keep this up to date
			for i := 1; i < maxNodesInAnyTest+1; i++ {
				node := fmt.Sprintf("node_%d_%d", stamp, i)

				existsErr := silentSystem("docker", "inspect", node)
				notExists := false
				if existsErr != nil {
					// must have been a single-node test, don't return on our
					// behalf, we have zpool etc cleanup to do
					notExists = true
				}

				err := system("docker", "exec", "-i", node, "test", "-e", "/CLEAN_ME_UP")
				if err != nil {
					fmt.Printf("not cleaning up %s because /CLEAN_ME_UP not found\n", node)
					if !notExists {
						return
					}
				}

				err = system("docker", "rm", "-f", node)
				if err != nil {
					fmt.Printf("erk during teardown %s\n", err)
				}

				// workaround https://github.com/docker/docker/issues/20398
				err = system("docker", "network", "disconnect", "-f", "bridge", node)
				if err != nil {
					fmt.Printf("erk during network force-disconnect %s\n", err)
				}

				// cleanup after a previous test run; this is a pretty gross hack
				err = system("bash", "-c", fmt.Sprintf(`
					for X in $(findmnt -P -R /tmpfs |grep %s); do
						eval $X
						if [ "$TARGET" != "/tmpfs" ]; then
							umount $TARGET >/dev/null 2>&1 || true
						fi
					done
					rm -rf /tmpfs/%s`, node, node),
				)
				if err != nil {
					fmt.Printf("erk during teardown %s\n", err)
				}

				fmt.Printf("=== Cleaned up node %s\n", node)
			}

			// clean up any leftover zpools
			out, err := exec.Command("zpool", "list", "-H").Output()
			if err != nil {
				fmt.Printf("unable to list zpools: %s\n", err)
			}
			shrap := strings.Split(string(out), "\n")
			for _, s := range shrap {
				shr := strings.Fields(string(s))
				if len(shr) > 0 {
					// Manually umount them and disregard failures
					if strings.HasPrefix(shr[0], fmt.Sprintf("testpool_%d", stamp)) {
						o, _ := exec.Command("bash", "-c",
							fmt.Sprintf(
								"for X in `cat /proc/self/mounts|grep testpool_%d"+
									"|grep -v '/mnt '|cut -d ' ' -f 2`; do "+
									"umount -f $X || true;"+
									"done", stamp),
						).CombinedOutput()
						fmt.Printf("Attempted pre-cleanup output: %s\n", o)
						o, err = exec.Command("zpool", "destroy", "-f", shr[0]).CombinedOutput()
						if err != nil {
							fmt.Printf("error running zpool destroy %s: %s %s\n", shr[0], o, err)
							time.Sleep(1 * time.Second)
							o, err := exec.Command("zpool", "destroy", "-f", shr[0]).CombinedOutput()
							if err != nil {
								fmt.Printf("Failed second try: %s %s", err, o)
							}
						}
						fmt.Printf("=== Cleaned up zpool %s\n", shr[0])
					}
				}
			}
		}()
	}
}

func docker(node string, cmd string) (string, error) {
	c := exec.Command("docker", "exec", "-i", node, "sh", "-c", cmd)

	var b bytes.Buffer

	o := io.MultiWriter(&b, os.Stdout)
	e := io.MultiWriter(&b, os.Stderr)

	c.Stdout = o
	c.Stderr = e
	err := c.Run()
	return string(b.Bytes()), err

}

func dockerSystem(node string, cmd string) error {
	return system("docker", "exec", "-i", node, "sh", "-c", cmd)
}

func d(t *testing.T, node string, cmd string) {
	fmt.Printf("RUNNING on %s: %s\n", node, cmd)
	s, err := docker(node, cmd)
	if err != nil {
		t.Error(fmt.Errorf("%s while running %s on %s: %s", err, cmd, node, s))
	}
}

func s(t *testing.T, node string, cmd string) string {
	s, err := docker(node, cmd)
	if err != nil {
		t.Error(fmt.Errorf("%s while running %s on %s: %s", err, cmd, node, s))
	}
	return s
}

func localImage() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.local:80/lukemarsden/datamesh-server:pushpull", hostname)
}

func localEtcdImage() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.local:80/coreos/etcd:v3.0.15", hostname)
}

func localImageArgs() string {
	logSuffix := ""
	if os.Getenv("DISABLE_LOG_AGGREGATION") == "" {
		logSuffix = " --log 172.17.0.1"
	}
	traceSuffix := ""
	if os.Getenv("DISABLE_TRACING") == "" {
		traceSuffix = " --trace 172.17.0.1"
	}
	regSuffix := ""
	if os.Getenv("ALLOW_PUBLIC_REGISTRATION") != "" {
		fmt.Sprintf("Allowing public registration!\n")
		regSuffix = " --allow-public-registration"
	}
	return ("--image " + localImage() + " --etcd-image " + localEtcdImage() +
		" --docker-api-version 1.23 --discovery-url http://172.17.0.1:8087" +
		logSuffix + traceSuffix + regSuffix +
		" --assets-url-prefix http://localhost:4000/datamesh-website/" +
		" --allow-public-registration")
}

// TODO a test which exercise `dm cluster init --count 3` or so

func dockerRun(v ...string) string {
	// supports either 1 or 2 args. in 1-arg case, just takes a volume name.
	// in 2-arg case, takes volume name and arguments to pass to docker run.
	// in 3-arg case, third arg is image in "$(hostname).local:80/$image".
	// in 4-arg case, fourth arg is volume target.
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	image := "busybox"
	if len(v) == 3 {
		image = v[2]
	}
	path := "/foo"
	if len(v) == 4 {
		path = v[3]
	}
	if len(v) > 1 {
		return fmt.Sprintf(
			"docker run -i -v %s:%s --volume-driver dm %s %s.local:80/%s",
			v[0], path, v[1], hostname, image,
		)
	} else {
		return fmt.Sprintf(
			"docker run -i -v %s:%s --volume-driver dm %s.local:80/%s",
			v[0], path, hostname, image,
		)
	}
}

var uniqNumber int

func uniqName() string {
	uniqNumber++
	return fmt.Sprintf("volume_%d", uniqNumber)
}

func TestSingleNode(t *testing.T) {
	// single node tests
	teardownFinishedTestRuns()

	startTiming()
	now := time.Now().UnixNano()
	err := testSetup(1, now)
	defer testMarkForCleanup(1, now)

	if err != nil {
		t.Error(err)
	}
	poolId := fmt.Sprintf("testpool_%d_1", now)
	node1 := fmt.Sprintf("node_%d_1", now)

	d(t, node1, "dm cluster init "+localImageArgs()+
		" --use-pool-dir /datamesh-test-pools/"+poolId+
		" --use-pool-name "+poolId,
	)

	time.Sleep(time.Second * 5)

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

	startTiming()
	now := time.Now().UnixNano()
	err := testSetup(2, now)
	defer testMarkForCleanup(2, now)
	if err != nil {
		t.Error(err)
	}
	logTiming("setup")
	node1 := fmt.Sprintf("node_%d_1", now)
	node2 := fmt.Sprintf("node_%d_2", now)
	poolId1 := fmt.Sprintf("testpool_%d_1", now)
	poolId2 := fmt.Sprintf("testpool_%d_2", now)

	st, err := docker(
		node1, "dm cluster init "+localImageArgs()+
			" --use-pool-dir /datamesh-test-pools/"+poolId1+
			" --use-pool-name "+poolId1,
	)
	if err != nil {
		t.Error(err)
	}
	lines := strings.Split(st, "\n")
	joinUrl := func(lines []string) string {
		for _, line := range lines {
			shrap := strings.Fields(line)
			if len(shrap) > 3 {
				if shrap[0] == "dm" && shrap[1] == "cluster" && shrap[2] == "join" {
					return shrap[3]
				}
			}
		}
		return ""
	}(lines)
	if joinUrl == "" {
		t.Error("unable to find join url in 'dm cluster init' output")
	}
	logTiming("init")
	_, err = docker(node2, fmt.Sprintf(
		"dm cluster join %s %s %s",
		localImageArgs()+" --use-pool-dir /datamesh-test-pools/"+poolId2,
		joinUrl,
		" --use-pool-name "+poolId2,
	))
	if err != nil {
		t.Error(err)
	}
	logTiming("join")
	dumpTiming()

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

	startTiming()
	now := time.Now().UnixNano()
	err := testSetup(2, now)
	defer testMarkForCleanup(2, now)
	if err != nil {
		t.Error(err)
	}
	logTiming("setup")

	node1 := fmt.Sprintf("node_%d_1", now)
	node2 := fmt.Sprintf("node_%d_2", now)
	poolId1 := fmt.Sprintf("testpool_%d_1", now)
	poolId2 := fmt.Sprintf("testpool_%d_2", now)

	_, err = docker(
		node1, "dm cluster init "+localImageArgs()+
			" --use-pool-dir /datamesh-test-pools/"+poolId1+
			" --use-pool-name "+poolId1,
	)
	if err != nil {
		t.Error(err)
	}
	logTiming("init1")

	_, err = docker(
		node2, "dm cluster init "+localImageArgs()+
			" --use-pool-dir /datamesh-test-pools/"+poolId2+
			" --use-pool-name "+poolId2,
	)
	if err != nil {
		t.Error(err)
	}
	logTiming("init2")

	node1IP := s(t,
		node1,
		`ifconfig eth0 | grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1`,
	)
	fmt.Printf("IP of node1: %s\n", node1IP)

	config := s(t,
		node1,
		"cat /root/.datamesh/config",
	)
	fmt.Printf("dm config on node1: %s\n", config)

	m := struct {
		Remotes struct{ Local struct{ ApiKey string } }
	}{}
	json.Unmarshal([]byte(config), &m)

	type Node struct {
		Name      string
		Container string
		IP        string
		ApiKey    string
	}
	type Pair struct {
		From Node
		To   Node
	}
	node1node := Node{
		Name:      "node1",
		Container: node1,
		IP:        node1IP,
		ApiKey:    m.Remotes.Local.ApiKey,
	}

	node2IP := s(t,
		node2,
		`ifconfig eth0 | grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1`,
	)
	fmt.Printf("IP of node2: %s\n", node2IP)

	config = s(t,
		node2,
		"cat /root/.datamesh/config",
	)
	fmt.Printf("dm config on node2: %s\n", config)

	// re-use m
	json.Unmarshal([]byte(config), &m)
	node2node := Node{
		Name:      "node2",
		Container: node2,
		IP:        node2IP,
		ApiKey:    m.Remotes.Local.ApiKey,
	}

	remoteAdd := func(t *testing.T) {
		for _, pair := range []Pair{
			Pair{From: node1node, To: node2node},
			Pair{From: node2node, To: node1node}} {
			found := false
			for _, remote := range strings.Split(s(t, pair.From.Container, "dm remote"), "\n") {
				if remote == pair.To.Name {
					found = true
				}
			}
			if !found {
				d(t, pair.From.Container, fmt.Sprintf(
					"echo %s |dm remote add %s admin@%s",
					pair.To.ApiKey,
					pair.To.Name,
					pair.To.IP,
				))
				res := s(t, pair.From.Container, "dm remote -v")
				if !strings.Contains(res, pair.To.Name) {
					t.Errorf("can't find %s in %s's remote config", pair.To.Name, pair.From.Name)
				}
				d(t, pair.From.Container, "dm remote switch local")
			}
		}
	}

	t.Run("RemoteAdd", func(t *testing.T) {
		remoteAdd(t)
	})

	t.Run("PushCommitBranchExtantBase", func(t *testing.T) {
		remoteAdd(t)
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push node1")

		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message remote's log output")
		}
		// test incremental push
		d(t, node2, "dm commit -m 'again'")
		d(t, node2, "dm push node1")

		resp = s(t, node1, "dm log")
		if !strings.Contains(resp, "again") {
			t.Error("unable to find commit message remote's log output")
		}
		// test pushing branch with extant base
		d(t, node2, "dm checkout -b newbranch")
		d(t, node2, "dm commit -m 'branchy'")
		d(t, node2, "dm push node1")

		d(t, node1, "dm checkout newbranch")
		resp = s(t, node1, "dm log")
		if !strings.Contains(resp, "branchy") {
			t.Error("unable to find commit message remote's log output")
		}
	})
	t.Run("PushCommitBranchNoExtantBase", func(t *testing.T) {
		remoteAdd(t)
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
		d(t, node2, "dm push node1")

		d(t, node1, "dm switch "+fsname)
		d(t, node1, "dm checkout newbranch3")
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "branchy3") {
			t.Error("unable to find commit message remote's log output")
		}
	})
	t.Run("DirtyDetected", func(t *testing.T) {
		remoteAdd(t)
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push node1")

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
		result := s(t, node2, "dm push node1 || true") // an error code is ok

		if !strings.Contains(result, "uncommitted") {
			t.Error("pushing didn't fail when there were known uncommited changes on the peer")
		}
	})
	t.Run("DirtyImmediate", func(t *testing.T) {
		remoteAdd(t)
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push node1")

		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message remote's log output")
		}
		// now dirty the filesystem on node1 w/1MB before it can be received into
		d(t, node1, dockerRun(""+fsname+"")+" dd if=/dev/urandom of=/foo/Y bs=1024 count=1024")

		// test incremental push
		d(t, node2, "dm commit -m 'again'")
		result := s(t, node2, "dm push node1 || true") // an error code is ok

		if !strings.Contains(result, "has been modified") {
			t.Error(
				"pushing didn't fail when there were known uncommited changes on the peer",
			)
		}
	})
	t.Run("Diverged", func(t *testing.T) {
		remoteAdd(t)
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push node1")

		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			t.Error("unable to find commit message remote's log output")
		}
		// now make a commit that will diverge the filesystems
		d(t, node1, "dm commit -m 'node1 commit'")

		// test incremental push
		d(t, node2, "dm commit -m 'node2 commit'")
		result := s(t, node2, "dm push node1 || true") // an error code is ok

		if !strings.Contains(result, "diverged") && !strings.Contains(result, "hello") {
			t.Error(
				"pushing didn't fail when there was a divergence",
			)
		}
	})
	t.Run("ResetAfterPushThenPushMySQL", func(t *testing.T) {
		remoteAdd(t)
		fsname := uniqName()
		d(t, node2, dockerRun(
			fsname, "-d -e MYSQL_ROOT_PASSWORD=secret", "mysql:5.7.17", "/var/lib/mysql",
		))
		time.Sleep(10 * time.Second)
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		d(t, node2, "dm push node1")

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
		d(t, node2, "dm push node1")
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
		remoteAdd(t)
		fsname := uniqName()
		d(t, node2, dockerRun(fsname)+" touch /foo/X")
		d(t, node2, "dm switch "+fsname)
		d(t, node2, "dm commit -m 'hello'")
		// XXX 'dm clone' currently tries to pull the named filesystem into the
		// _current active filesystem name_. instead, it should pull it into a
		// new filesystem with the same name. if the same named filesystem
		// already exists, it should error (and instruct the user to 'dm switch
		// foo; dm pull foo' instead).
		d(t, node1, "dm clone node2 "+fsname)
		d(t, node1, "dm switch "+fsname)
		resp := s(t, node1, "dm log")
		if !strings.Contains(resp, "hello") {
			// TODO fix this failure by sending prelude in intercluster case also
			t.Error("unable to find commit message remote's log output")
		}
		/*
			// test incremental pull
			d(t, node2, "dm commit -m 'again'")
			d(t, node1, "dm pull node1 "+fsname)

			resp = s(t, node1, "dm log")
			if !strings.Contains(resp, "again") {
				t.Error("unable to find commit message remote's log output")
			}
			// test pulling branch with extant base
			d(t, node2, "dm checkout -b newbranch")
			d(t, node2, "dm commit -m 'branchy'")
			d(t, node1, "dm pull node1 "+fsname+" newbranch")

			d(t, node1, "dm checkout newbranch")
			resp = s(t, node1, "dm log")
			if !strings.Contains(resp, "branchy") {
				t.Error("unable to find commit message remote's log output")
			}
		*/
	})
}

// TODO: spin up _three_ single node clusters, use one as a hub so that alice
// and bob can collaborate.
