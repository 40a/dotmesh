package main

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
		for _, n := range c.Nodes {
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
		# tmpfs makes etcd not completely rinse your IOPS (which it can do
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

func localFrontendTestRunnerImage() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.local:80/lukemarsden/datamesh-frontend-test-runner:pushpull", hostname)
}

func localChromeDriverImage() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.local:80/lukemarsden/datamesh-chromedriver:pushpull", hostname)
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
	return fmt.Sprintf("cluster-%d-%d-node-%d", now, i, j)
}

func poolId(now int64, i, j int) string {
	return fmt.Sprintf("testpool-%d-%d-node-%d", now, i, j)
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
