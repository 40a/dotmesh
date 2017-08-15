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

You need to be running a local registry, as well as everything else in the
github.com/lukemarsden/datamesh-instrumentation pack, which requires
docker-compose (run up.sh with a password as the first argument).

Finally, you need to be running github.com/lukemarsden/discovery.datamesh.io
on port 8087:

	git clone git@github.com:lukemarsden/discovery.datamesh.io
	cd discovery.datamesh.io
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

Now install some deps (for tests only; as root):

	go get github.com/tools/godep
	apt install zfsutils-linux jq
	echo 'vm.max_map_count=262144' >> /etc/sysctl.conf
	sysctl vm.max_map_count=262144

You can now run tests, like:

	./mark-cleanup.sh; ./rebuild.sh && ./test.sh -run TestTwoSingleNodeClusters

To open a bunch of debug tools, run (where 'secret' is the pasword you
specified when you ran 'up.sh' in datamesh-instrumentation):

	ADMIN_PW=secret ./creds.sh
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func testMarkForCleanup(f Federation) {
	for _, c := range f {
		for _, n := range c.Nodes {
			node := n.Container
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
}

// this starts a chromedriver container
// it expects a datamesh-server-inner to be running
func startChromeDriver() error {

}

func testSetup(f Federation, stamp int64) error {
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

	for i, c := range f {
		for j := 0; j < c.DesiredNodeCount; j++ {
			node := nodeName(stamp, i, j)
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
	}
	return nil
}

type N struct {
	Timestamp  int64
	ClusterNum string
	NodeNum    string
}

func teardownFinishedTestRuns() {
	cs, err := exec.Command(
		"docker", "ps", "--filter", "name=^/cluster_.*$", "--format", "{{.Names}}",
	).Output()
	if err != nil {
		panic(err)
	}
	stamps := map[int64][]N{}
	for _, line := range strings.Split(string(cs), "\n") {
		shrap := strings.Split(line, "_")
		if len(shrap) > 4 {
			// cluster_<timestamp>_<clusterNum>_node_<nodeNum>
			stamp := shrap[1]
			clusterNum := shrap[2]
			nodeNum := shrap[4]

			i, err := strconv.ParseInt(stamp, 10, 64)
			if err != nil {
				panic(err)
			}
			_, ok := stamps[i]
			if !ok {
				stamps[i] = []N{}
			}
			stamps[i] = append(stamps[i], N{
				Timestamp:  i,
				ClusterNum: clusterNum,
				NodeNum:    nodeNum,
			})
		}
	}

	for stamp, ns := range stamps {
		func() {
			for _, n := range ns {
				cn, err := strconv.Atoi(n.ClusterNum)
				if err != nil {
					fmt.Printf("can't deduce clusterNum: %s", cn)
					return
				}

				nn, err := strconv.Atoi(n.NodeNum)
				if err != nil {
					fmt.Printf("can't deduce nodeNum: %s", nn)
					return
				}

				node := nodeName(stamp, cn, nn)
				existsErr := silentSystem("docker", "inspect", node)
				notExists := false
				if existsErr != nil {
					// must have been a single-node test, don't return on our
					// behalf, we have zpool etc cleanup to do
					notExists = true
				}

				err = system("docker", "exec", "-i", node, "test", "-e", "/CLEAN_ME_UP")
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

	f := Federation{NewCluster(1)}

	startTiming()
	err := f.Start(t)
	defer testMarkForCleanup(f)
	if err != nil {
		t.Error(err)
	}
	node1 := f[0].Nodes[0].Container

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

	node1 := f[0].Nodes[0].Container
	node2 := f[0].Nodes[1].Container

	t.Run("Move", func(t *testing.T) {
		fsname := uniqName()
		d(t, node1, dockerRun(fsname)+" sh -c 'echo WORLD > /foo/HELLO'")
		st := s(t, node2, dockerRun(fsname)+" cat /foo/HELLO")
		if !strings.Contains(st, "WORLD") {
			t.Error(fmt.Sprintf("Unable to find world in transported data capsule, got '%s'", st))
		}
	})
}

type Node struct {
	ClusterName string
	Container   string
	IP          string
	ApiKey      string
}

type Cluster struct {
	DesiredNodeCount int
	Nodes            []Node
}

type Pair struct {
	From Node
	To   Node
}

func NewCluster(desiredNodeCount int) *Cluster {
	return &Cluster{DesiredNodeCount: desiredNodeCount}
}

type Federation []*Cluster

func nodeName(now int64, i, j int) string {
	return fmt.Sprintf("cluster_%d_%d_node_%d", now, i, j)
}

func poolId(now int64, i, j int) string {
	return fmt.Sprintf("testpool_%d_%d_node_%d", now, i, j)
}

func NodeFromNodeName(t *testing.T, now int64, i, j int, clusterName string) Node {
	nodeIP := s(t,
		nodeName(now, i, j),
		`ifconfig eth0 | grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1`,
	)
	config := s(t,
		nodeName(now, i, j),
		"cat /root/.datamesh/config",
	)
	fmt.Printf("dm config on %s: %s\n", nodeName(now, i, j), config)

	m := struct {
		Remotes struct{ Local struct{ ApiKey string } }
	}{}
	json.Unmarshal([]byte(config), &m)

	return Node{
		ClusterName: clusterName,
		Container:   nodeName(now, i, j),
		IP:          nodeIP,
		ApiKey:      m.Remotes.Local.ApiKey,
	}
}

func (f Federation) Start(t *testing.T) error {
	teardownFinishedTestRuns()

	startTiming()
	now := time.Now().UnixNano()
	err := testSetup(f, now)
	defer testMarkForCleanup(f)
	if err != nil {
		return err
	}
	logTiming("setup")

	for i, c := range f {
		// init the first node in the cluster, join the rest
		if c.DesiredNodeCount == 0 {
			panic("no such thing as a zero-node cluster")
		}
		st, err := docker(
			nodeName(now, i, 0), "dm cluster init "+localImageArgs()+
				" --use-pool-dir /datamesh-test-pools/"+poolId(now, i, 0)+
				" --use-pool-name "+poolId(now, i, 0),
		)
		if err != nil {
			return err
		}
		clusterName := fmt.Sprintf("cluster_%d", i)
		c.Nodes = append(c.Nodes, NodeFromNodeName(t, now, i, 0, clusterName))

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
			return fmt.Errorf("unable to find join url in 'dm cluster init' output")
		}
		logTiming("init_" + poolId(now, i, 0))
		for j := 1; j < c.DesiredNodeCount; j++ {
			// if c.Nodes is 3, this iterates over 1 and 2 (0 was the init'd
			// node).
			_, err = docker(nodeName(now, i, j), fmt.Sprintf(
				"dm cluster join %s %s %s",
				localImageArgs()+" --use-pool-dir /datamesh-test-pools/"+poolId(now, i, j),
				joinUrl,
				" --use-pool-name "+poolId(now, i, j),
			))
			if err != nil {
				return err
			}
			c.Nodes = append(c.Nodes, NodeFromNodeName(t, now, i, j, clusterName))

			logTiming("join_" + poolId(now, i, j))
		}
	}
	// TODO refactor the following so that each node has one other node on the
	// other cluster as a remote named 'cluster0' or 'cluster1', etc.

	// for each node in each cluster, add remotes for all the other clusters
	// O(n^3)
	pairs := []Pair{}
	for _, c := range f {
		for _, node := range c.Nodes {
			for _, otherCluster := range f {
				first := otherCluster.Nodes[0]
				pairs = append(pairs, Pair{
					From: node,
					To:   first,
				})
			}
		}
	}
	for _, pair := range pairs {
		found := false
		for _, remote := range strings.Split(s(t, pair.From.Container, "dm remote"), "\n") {
			if remote == pair.To.ClusterName {
				found = true
			}
		}
		if !found {
			d(t, pair.From.Container, fmt.Sprintf(
				"echo %s |dm remote add %s admin@%s",
				pair.To.ApiKey,
				pair.To.ClusterName,
				pair.To.IP,
			))
			res := s(t, pair.From.Container, "dm remote -v")
			if !strings.Contains(res, pair.To.ClusterName) {
				t.Errorf("can't find %s in %s's remote config", pair.To.ClusterName, pair.From.ClusterName)
			}
			d(t, pair.From.Container, "dm remote switch local")
		}
	}
	return nil
}

func TestTwoSingleNodeClusters(t *testing.T) {

	f := Federation{
		NewCluster(1), // cluster_0_node_0
		NewCluster(1), // cluster_1_node_0
	}
	err := f.Start(t)
	defer testMarkForCleanup(f)
	if err != nil {
		t.Error(err)
	}
	node1 := f[0].Nodes[0].Container
	node2 := f[1].Nodes[0].Container

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

// TODO: spin up _three_ single node clusters, use one as a hub so that alice
// and bob can collaborate.

// TODO: run dind/dind-cluster.sh up, and then test the manifests in
// kubernetes/ against the resulting (3 node by default) cluster. Ensure things
// run offline. Figure out how to configure each cluster node with its own
// zpool. Test dynamic provisioning, and so on.


func TestFrontend(t *testing.T) {
	// single node tests
	teardownFinishedTestRuns()

	f := Federation{NewCluster(1)}

	startTiming()
	err := f.Start(t)
	defer testMarkForCleanup(f)
	if err != nil {
		t.Error(err)
	}
	node1 := f[0].Nodes[0].Container

	t.Run("Authenticate", func(t *testing.T) {
		fsname := uniqName()
		err := system("bash", "-c", `
			docker ps -a
		`)
		if err != nil {
			t.Error(fmt.Sprintf("there was an error %v", "err"))
			return err
		}
	})

}
