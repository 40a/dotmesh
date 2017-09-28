package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/rpc/v2/json2"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

var timings map[string]float64
var lastTiming int64

const HOST_IP_FROM_CONTAINER = "10.192.0.1"

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

func tryUntilSucceeds(f func() error, desc string) error {
	attempt := 0
	for {
		err := f()
		if err != nil {
			if attempt > 5 {
				return err
			} else {
				fmt.Printf("Error %s: %v, pausing and trying again...\n", desc, err)
				time.Sleep(time.Duration(attempt) * time.Second)
			}
		} else {
			return nil
		}
		attempt++
	}
}

func testMarkForCleanup(f Federation) {
	for _, c := range f {
		for _, n := range c.GetNodes() {
			node := n.Container
			err := tryUntilSucceeds(func() error {
				return system("bash", "-c", fmt.Sprintf(
					`docker exec -t %s bash -c 'touch /CLEAN_ME_UP'`, node,
				))
			}, fmt.Sprintf("marking %s for cleanup", node))
			if err != nil {
				fmt.Printf("Error marking %s for cleanup: %s, giving up.\n", node, err)
				panic("This is bad. Stop everything and clean up manually!")
			}
		}
	}
}

func testSetup(f Federation, stamp int64) error {
	err := system("bash", "-c", `
		# Create a home for the test pools to live that can have the same path
		# both from ZFS's perspective and that of the inner container.
		# (Bind-mounts all the way down.)
		mkdir -p /datamesh-test-pools
	`)
	if err != nil {
		return err
	}

	for i, c := range f {
		for j := 0; j < c.GetDesiredNodeCount(); j++ {
			node := nodeName(stamp, i, j)
			fmt.Printf(">>> Using RunArgs %s\n", c.RunArgs(j))
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
			EXTRA_DOCKER_ARGS="-v /datamesh-test-pools:/datamesh-test-pools:rshared" \
			DIND_IMAGE="quay.io/lukemarsden/kubeadm-dind-cluster:v1.7-hostport" \
			CNI_PLUGIN=weave \
				../kubernetes/dind-cluster-v1.7.sh bare $NODE %s
			sleep 1
			docker exec -t $NODE bash -c '
			    echo "%s '$(hostname)'.local" >> /etc/hosts
				sed -i "s/rundocker/rundocker \
					--insecure-registry '$(hostname)'.local:80/" \
					/etc/systemd/system/docker.service.d/20-fs.conf
				systemctl daemon-reload
				systemctl restart docker
			'
			docker cp ../binaries/Linux/dm $NODE:/usr/local/bin/dm
			`, node, c.RunArgs(j), HOST_IP_FROM_CONTAINER))
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
		"bash", "-c", "docker ps --format {{.Names}} |grep cluster- || true",
	).Output()
	if err != nil {
		panic(err)
	}
	stamps := map[int64][]N{}
	for _, line := range strings.Split(string(cs), "\n") {
		shrap := strings.Split(line, "-")
		if len(shrap) > 4 {
			// cluster-<timestamp>-<clusterNum>-node-<nodeNum>
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
					if strings.HasPrefix(shr[0], fmt.Sprintf("testpool-%d", stamp)) {
						o, _ := exec.Command("bash", "-c",
							fmt.Sprintf(
								"for X in `cat /proc/self/mounts|grep testpool-%d"+
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
	err = system("docker", "container", "prune", "-f")
	if err != nil {
		fmt.Printf("Error from docker container prune -f: %v", err)
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
	return fmt.Sprintf("%s.local:80/datamesh/datamesh-server:latest", hostname)
}

func localFrontendTestRunnerImage() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.local:80/datamesh/datamesh-frontend-test-runner:latest", hostname)
}

func localChromeDriverImage() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.local:80/datamesh/datamesh-chromedriver:latest", hostname)
}

func localEtcdImage() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.local:80/datamesh/etcd:v3.0.15", hostname)
}

func localImageArgs() string {
	logSuffix := ""
	if os.Getenv("DISABLE_LOG_AGGREGATION") == "" {
		logSuffix = fmt.Sprintf(" --log %s", HOST_IP_FROM_CONTAINER)
	}
	traceSuffix := ""
	if os.Getenv("DISABLE_TRACING") == "" {
		traceSuffix = fmt.Sprintf(" --trace %s", HOST_IP_FROM_CONTAINER)
	}
	regSuffix := ""
	if os.Getenv("ALLOW_PUBLIC_REGISTRATION") != "" {
		fmt.Sprintf("Allowing public registration!\n")
		regSuffix = " --allow-public-registration"
	}
	return ("--image " + localImage() + " --etcd-image " + localEtcdImage() +
		" --docker-api-version 1.23 --discovery-url http://" + HOST_IP_FROM_CONTAINER + ":8087" +
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

type Kubernetes struct {
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

func NewKubernetes(desiredNodeCount int) *Kubernetes {
	return &Kubernetes{DesiredNodeCount: desiredNodeCount}
}

type Federation []Startable

func nodeName(now int64, i, j int) string {
	return fmt.Sprintf("cluster-%d-%d-node-%d", now, i, j)
}

func poolId(now int64, i, j int) string {
	return fmt.Sprintf("testpool-%d-%d-node-%d", now, i, j)
}

func NodeFromNodeName(t *testing.T, now int64, i, j int, clusterName string) Node {
	nodeIP := strings.TrimSpace(s(t,
		nodeName(now, i, j),
		`ifconfig eth0 | grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1`,
	))
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
	now := time.Now().UnixNano()
	err := testSetup(f, now)
	if err != nil {
		return err
	}
	logTiming("setup")

	for i, c := range f {
		fmt.Printf("==== GOING FOR %d, %+v ====\n", i, c)
		err = c.Start(t, now, i)
		if err != nil {
			return err
		}
	}
	// TODO refactor the following so that each node has one other node on the
	// other cluster as a remote named 'cluster0' or 'cluster1', etc.

	// for each node in each cluster, add remotes for all the other clusters
	// O(n^3)
	pairs := []Pair{}
	for _, c := range f {
		for _, node := range c.GetNodes() {
			for _, otherCluster := range f {
				first := otherCluster.GetNode(0)
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

type Startable interface {
	GetNode(int) Node
	GetNodes() []Node
	GetDesiredNodeCount() int
	Start(*testing.T, int64, int) error
	RunArgs(int) string
}

///////////// Kubernetes

func (c *Kubernetes) RunArgs(i int) string {
	// special args for starting Kube clusters, copying observed behaviour of
	// dind::up
	if i == 0 {
		return fmt.Sprintf("10.192.0.%d %d 127.0.0.1:8080:8080", i+2, i+1)
	} else {
		return fmt.Sprintf("10.192.0.%d %d ''", i+2, i+1)
	}
}

func (c *Kubernetes) GetNode(i int) Node {
	return c.Nodes[i]
}

func (c *Kubernetes) GetNodes() []Node {
	return c.Nodes
}

func (c *Kubernetes) GetDesiredNodeCount() int {
	return c.DesiredNodeCount
}

func (c *Kubernetes) Start(t *testing.T, now int64, i int) error {
	if c.DesiredNodeCount == 0 {
		panic("no such thing as a zero-node cluster")
	}

	images, err := ioutil.ReadFile("../kubernetes/images.txt")
	if err != nil {
		return err
	}
	cache := map[string]string{}
	for _, x := range strings.Split(string(images), "\n") {
		ys := strings.Split(x, " ")
		if len(ys) == 2 {
			cache[ys[0]] = ys[1]
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	// pre-pull all the container images Kubernetes needs to use, tag them to
	// trick it into not downloading anything.
	finishing := make(chan bool)
	for j := 0; j < c.DesiredNodeCount; j++ {
		go func(j int) {
			// Use the locally build datamesh server image as the "latest" image in
			// the test containers.
			st, err := docker(
				nodeName(now, i, j),
				fmt.Sprintf(
					"docker pull %s.local:80/datamesh/datamesh-server:latest && "+
						"docker tag %s.local:80/datamesh/datamesh-server:latest "+
						"quay.io/datamesh/datamesh-server:latest",
					hostname, hostname,
				),
			)
			if err != nil {
				panic(st)
			}
			for fqImage, localName := range cache {
				st, err := docker(
					nodeName(now, i, j),
					/*
					   docker pull $local_name
					   docker tag $local_name $fq_image
					*/
					fmt.Sprintf(
						"docker pull %s.local:80/%s && "+
							"docker tag %s.local:80/%s %s",
						hostname, localName, hostname, localName, fqImage,
					),
				)
				if err != nil {
					panic(st)
				}
			}
			finishing <- true
		}(j)
	}
	for j := 0; j < c.DesiredNodeCount; j++ {
		_ = <-finishing
	}

	// TODO regex the following yamels to refer to the newly pushed
	// datamesh container image, rather than the latest stable
	err = system("bash", "-c",
		fmt.Sprintf(
			`MASTER=%s
			docker exec $MASTER mkdir /datamesh-kube-yaml
			for X in ../kubernetes/*.yaml; do docker cp $X $MASTER:/datamesh-kube-yaml/; done
			docker exec $MASTER sed -i 's/quay.io\/datamesh\/datamesh-server:latest/'$(hostname)'.local:80\/datamesh\/datamesh-server:latest/' /datamesh-kube-yaml/datamesh-ds.yaml
			docker exec $MASTER sed -i 's/pool/%s/' /datamesh-kube-yaml/datamesh-ds.yaml
			docker exec $MASTER sed -i 's/\/var\/lib\/docker\/datamesh/%s/' /datamesh-kube-yaml/datamesh-ds.yaml
			`,
			nodeName(now, i, 0),
			// need to somehow number the instances, did this by modifying
			// require_zfs.sh to include the hostname in the pool name to make
			// them unique... TODO: make sure we clear these up
			poolId(now, i, 0),
			"\\/datamesh-test-pools\\/"+poolId(now, i, 0),
		),
	)
	if err != nil {
		return err
	}
	st, err := docker(
		nodeName(now, i, 0),
		"rm /etc/machine-id && systemd-machine-id-setup && "+
			"systemctl start kubelet && "+
			"kubeadm init --kubernetes-version=v1.7.6 --pod-network-cidr=10.244.0.0/16 --skip-preflight-checks && "+
			"mkdir /root/.kube && cp /etc/kubernetes/admin.conf /root/.kube/config && "+
			// Make kube-dns faster; trick copied from dind-cluster-v1.7.sh
			"kubectl get deployment kube-dns -n kube-system -o json | jq '.spec.template.spec.containers[0].readinessProbe.initialDelaySeconds = 3|.spec.template.spec.containers[0].readinessProbe.periodSeconds = 3' | kubectl apply --force -f -",
	)
	if err != nil {
		return err
	}

	lines := strings.Split(st, "\n")

	joinArgs := func(lines []string) string {
		for _, line := range lines {
			shrap := strings.Fields(line)
			if len(shrap) > 3 {
				// line will look like:
				//     kubeadm join --token c06d9b.57ef131db5c0e0e5 10.192.0.2:6443
				if shrap[0] == "kubeadm" && shrap[1] == "join" {
					return strings.Join(shrap[2:], " ")
				}
			}
		}
		return ""
	}(lines)

	fmt.Printf("JOIN URL: %s\n", joinArgs)

	clusterName := fmt.Sprintf("cluster_%d", i)

	for j := 1; j < c.DesiredNodeCount; j++ {
		// if c.Nodes is 3, this iterates over 1 and 2 (0 was the init'd
		// node).
		_, err = docker(nodeName(now, i, j), fmt.Sprintf(
			"rm /etc/machine-id && systemd-machine-id-setup && "+
				"systemctl start kubelet && "+
				"kubeadm join --skip-preflight-checks %s",
			joinArgs,
		))
		if err != nil {
			return err
		}
		logTiming("join_" + poolId(now, i, j))
	}
	// now install datamesh yaml (setting initial admin pw)
	st, err = docker(
		nodeName(now, i, 0),
		"kubectl apply -f /datamesh-kube-yaml/weave-net.yaml && "+
			"kubectl create namespace datamesh && "+
			"echo 'secret123' > datamesh-admin-password.txt && "+
			"kubectl create secret generic datamesh "+
			"    --from-file=datamesh-admin-password.txt -n datamesh && "+
			"rm datamesh-admin-password.txt && "+
			// install datamesh once on the master (retry because etcd operator
			// needs to initialize)
			"while ! kubectl apply -f /datamesh-kube-yaml; do sleep 1; done",
	)
	if err != nil {
		return err
	}
	// Add the nodes at the end, because NodeFromNodeName expects datamesh
	// config to be set up.
	for j := 0; j < c.DesiredNodeCount; j++ {
		st, err = docker(
			nodeName(now, i, j),
			"while ! (echo secret123 | dm remote add local admin@127.0.0.1); "+
				"    do echo 'retrying...' && sleep 1; done",
		)
		if err != nil {
			return err
		}
		c.Nodes = append(c.Nodes, NodeFromNodeName(t, now, i, j, clusterName))
	}
	return nil
}

///////////// Cluster (plain Datamesh cluster, no orchestrator)

func (c *Cluster) RunArgs(i int) string {
	// No special args required for dind with plain Datamesh.
	return ""
}

func (c *Cluster) GetNode(i int) Node {
	return c.Nodes[i]
}

func (c *Cluster) GetNodes() []Node {
	return c.Nodes
}

func (c *Cluster) GetDesiredNodeCount() int {
	return c.DesiredNodeCount
}

func (c *Cluster) Start(t *testing.T, now int64, i int) error {
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
	fmt.Printf("(just added) Here are my nodes: %+v\n", c.Nodes)

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
	return nil
}

func startChromeDriver(t *testing.T, node string) {
	chromeDriverImage := localChromeDriverImage()
	d(t, node, fmt.Sprintf(`
		docker run -d \
			--name datamesh-chromedriver \
			--link datamesh-server-inner:server \
			-e VNC_ENABLED=true \
			-e EXPOSE_X11=true \
			%s
	`, chromeDriverImage))
}

func stopChromeDriver(t *testing.T, node string) {
	d(t, node, "docker rm -f datamesh-chromedriver || true")
}

type UserLogin struct {
	Email    string
	Username string
	Password string
}

var uniqUserNumber int

func uniqLogin() UserLogin {
	uniqUserNumber++
	return UserLogin{
		Email:    fmt.Sprintf("test%d@test.com", uniqUserNumber),
		Username: fmt.Sprintf("test%d", uniqUserNumber),
		Password: "test",
	}
}

// run the frontend tests - then copy the media out onto the dind host
func runFrontendTest(t *testing.T, node string, testName string, login UserLogin) {
	runnerImage := localFrontendTestRunnerImage()
	d(t, node, fmt.Sprintf(`
		docker run --rm \
	    --name datamesh-frontend-test-runner \
	    --link "datamesh-server-inner:server" \
	    --link "datamesh-chromedriver:chromedriver" \
	    -e "LAUNCH_URL=server:6969/ui" \
	    -e "SELENIUM_HOST=chromedriver" \
	    -e "WAIT_FOR_HOSTS=server:6969 chromedriver:4444 chromedriver:6060" \
	    -e "TEST_USER=%s" \
	    -e "TEST_EMAIL=%s" \
	    -e "TEST_PASSWORD=%s" \
	    -v /test_media/screenshots:/home/node/screenshots \
	    -v /test_media/videos:/home/node/videos \
	    %s %s
	  ls -la /test_media/screenshots
	  ls -la /test_media/videos
	`,
		login.Username,
		login.Email,
		login.Password,
		runnerImage,
		testName,
	))
}

func copyMedia(node string) error {
	err := system("bash", "-c", fmt.Sprintf(`
		docker exec %s bash -c "tar -C /test_media -c ." > ../frontend_artifacts.tar
	`, node))

	return err
}

func registerUser(ip, username, email, password string) error {
	fmt.Printf("Registering test user %s on node %s\n", username, ip)

	registerPayload := struct {
		Username string `json:"Name"`
		Email    string `json:"Email"`
		Password string `json:"Password"`
	}{
		Username: username,
		Email:    email,
		Password: password,
	}

	b, err := json.Marshal(registerPayload)
	if err != nil {
		return err
	}

	resp, err := http.Post(
		fmt.Sprintf("http://%s:6969/register", ip),
		"application/json",
		bytes.NewReader(b),
	)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("Invalid response from user registration request: %d: %v", resp.StatusCode, string(body))
	}

	return nil
}

func doRPC(hostname, user, apiKey, method string, args interface{}, result interface{}) error {
	url := fmt.Sprintf("http://%s:6969/rpc", hostname)
	message, err := json2.EncodeClientRequest(method, args)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(message))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(user, apiKey)
	client := new(http.Client)

	resp, err := client.Do(req)

	if err != nil {
		fmt.Printf("Test RPC FAIL: %+v -> %s -> %+v\n", args, method, err)
		return err
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Test RPC FAIL: %+v -> %s -> %+v\n", args, method, err)
		return fmt.Errorf("Error reading body: %s", err)
	}
	err = json2.DecodeClientResponse(bytes.NewBuffer(b), &result)
	if err != nil {
		fmt.Printf("Test RPC FAIL: %+v -> %s -> %+v\n", args, method, err)
		return fmt.Errorf("Couldn't decode response '%s': %s", string(b), err)
	}
	fmt.Printf("Test RPC: %+v -> %s -> %+v\n", args, method, result)
	return nil
}

func doAddCollaborator(hostname, user, apikey, namespace, volume, collaborator string) error {
	// FIXME: Duplicated types, see issue #44
	type VolumeName struct {
		Namespace string
		Name      string
	}

	var volumes map[string]map[string]struct {
		Id             string
		Name           VolumeName
		Clone          string
		Master         string
		SizeBytes      int64
		DirtyBytes     int64
		CommitCount    int64
		ServerStatuses map[string]string // serverId => status
	}

	err := doRPC(hostname, user, apikey,
		"DatameshRPC.List",
		struct {
		}{},
		&volumes)
	if err != nil {
		return err
	}

	volumeID := volumes[namespace][volume].Id

	var result bool
	err = doRPC(hostname, user, apikey,
		"DatameshRPC.AddCollaborator",
		struct {
			Volume, Collaborator string
		}{
			Volume:       volumeID,
			Collaborator: collaborator,
		},
		&result)
	if err != nil {
		return err
	}
	if !result {
		return fmt.Errorf("AddCollaborator failed without an error")
	}
	return nil
}
