package commands

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"github.com/blang/semver"
	"github.com/coreos/etcd/client"
	"github.com/datamesh-io/datamesh/cmd/dm/pkg/pki"
	"github.com/datamesh-io/datamesh/cmd/dm/pkg/remotes"
	"github.com/spf13/cobra"
)

const DATAMESH_DOCKER_IMAGE = "quay.io/datamesh/datamesh-server:latest"
const ADMIN_USER_UUID = "00000000-0000-0000-0000-000000000000"

var (
	serverCount              int
	traceAddr                string
	logAddr                  string
	allowPublicRegistrations bool
	etcdInitialCluster       string
	offline                  bool
	datameshDockerImage      string
	etcdDockerImage          string
	dockerApiVersion         string
	usePoolDir               string
	usePoolName              string
	discoveryUrl             string
	assetsURLPrefix          string
	homepageURL              string
	frontendStaticFolder     string
	configFile               string
)

var timings map[string]float64
var lastTiming int64

var logFile *os.File

func startTiming() {
	var err error
	logFile, err = os.Create("datamesh_install_log.txt")
	if err != nil {
		panic(err)
	}
	lastTiming = time.Now().UnixNano()
	timings = make(map[string]float64)
}

func logTiming(tag string) {
	now := time.Now().UnixNano()
	timings[tag] = float64(now-lastTiming) / (1000 * 1000 * 1000)
	lastTiming = now
}

func dumpTiming() {
	fmt.Fprintf(logFile, "=== TIMING ===\n")
	for tag, timing := range timings {
		fmt.Fprintf(logFile, "%s => %.2f\n", tag, timing)
	}
	fmt.Fprintf(logFile, "=== END TIMING ===\n")
	timings = map[string]float64{}
}

func NewCmdCluster(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Install a datamesh server on a docker host, creating or joining a cluster",
		Long: `Either initialize a new cluster or join an existing one.

Requires: Docker >= 1.10.0. Must be run on the same machine where the docker
daemon is running. (Also works on Docker for Mac.)

Run 'dm cluster init' on one node, and then 'dm cluster join <cluster-url>' on
another.`,
	}
	cmd.AddCommand(NewCmdClusterInit(os.Stdout))
	cmd.AddCommand(NewCmdClusterJoin(os.Stdout))
	cmd.AddCommand(NewCmdClusterReset(os.Stdout))
	cmd.AddCommand(NewCmdClusterUpgrade(os.Stdout))
	cmd.PersistentFlags().StringVar(
		&traceAddr, "trace", "",
		"Hostname for Zipkin host to enable distributed tracing",
	)
	cmd.PersistentFlags().StringVar(
		&logAddr, "log", "",
		"Hostname for datamesh logs to be forwarded to enable log aggregation",
	)
	cmd.PersistentFlags().BoolVar(
		&allowPublicRegistrations, "allow-public-registration", false,
		"Allow anyone who can connect to this datamesh cluster to create an account on it "+
			"at :6969/register",
	)
	cmd.PersistentFlags().StringVar(
		&datameshDockerImage, "image", DATAMESH_DOCKER_IMAGE,
		"datamesh-server docker image to use",
	)
	cmd.PersistentFlags().StringVar(
		&etcdDockerImage, "etcd-image",
		"quay.io/datamesh/etcd:v3.0.15",
		"etcd docker image to use",
	)
	cmd.PersistentFlags().StringVar(
		&dockerApiVersion, "docker-api-version",
		"", "specific docker API version to use, if you're using a < 1.12 "+
			"docker daemon and getting a 'client is newer than server' error in the "+
			"logs (specify the 'server API version' from the error message here)",
	)
	cmd.PersistentFlags().StringVar(
		&usePoolDir, "use-pool-dir",
		"", "directory in which to make a file-based-pool; useful for testing",
	)
	cmd.PersistentFlags().StringVar(
		&usePoolName, "use-pool-name",
		"", "name of pool to import or create; useful for testing",
	)
	cmd.PersistentFlags().StringVar(
		&discoveryUrl, "discovery-url",
		"https://discovery.datamesh.io", "URL of discovery service. "+
			"Use one you trust. Use HTTPS otherwise your private key will"+
			"be transmitted in plain text!",
	)
	cmd.PersistentFlags().BoolVar(
		&offline, "offline", false,
		"Do not attempt any operations that require internet access "+
			"(assumes datamesh-server docker image has already been pulled)",
	)
	cmd.PersistentFlags().StringVar(
		&assetsURLPrefix, "assets-url-prefix",
		"https://get.datamesh.io/assets/datamesh-website/_site",
		"Alternative URL prefix for assets for built-in datamesh JavaScript app",
	)
	cmd.PersistentFlags().StringVar(
		&homepageURL, "homepage-url", "https://datamesh.io/",
		"Alternative URL to use for homepage links in built-in JavaScript app",
	)
	cmd.PersistentFlags().StringVar(
		&frontendStaticFolder, "frontend-static-folder", "",
		"local folder to serve frontend assets - used in production",
	)
	cmd.PersistentFlags().StringVar(
		&configFile, "config-file", "/etc/datamesh/config.yaml",
		"datamesh config file (optional)", // TODO: document the config file in the docs!
	)
	return cmd
}

func NewCmdClusterInit(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a datamesh cluster",
		Run: func(cmd *cobra.Command, args []string) {
			err := clusterInit(cmd, args, out)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		},
	}
	cmd.Flags().IntVar(
		&serverCount, "count", 1,
		"Initial cluster size",
	)
	return cmd
}

func NewCmdClusterJoin(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join a node into an existing datamesh cluster",
		Run: func(cmd *cobra.Command, args []string) {
			err := clusterJoin(cmd, args, out)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		},
	}
	cmd.PersistentFlags().StringVar(
		&etcdInitialCluster, "etcd-initial-cluster", "",
		"Node was previously in etcd cluster, set this to the value of "+
			"'ETCD_INITIAL_CLUSTER' as given by 'etcdctl member add'",
	)
	return cmd
}

func NewCmdClusterUpgrade(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade a single node in an existing datamesh cluster",
		Run: func(cmd *cobra.Command, args []string) {
			err := clusterUpgrade(cmd, args, out)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		},
	}
	return cmd
}

func NewCmdClusterReset(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Uninstall datamesh from a docker host",
		Long: `Remove the datamesh-server and etcd containers. Deletes etcd data so that a new
cluster can be initialized. Does not delete any ZFS data (the data can be
'adopted' by a new cluster, but will lose name->filesystem data
associations since that 'registry' is stored in etcd). Also deletes cached
kernel modules. Also deletes certificates.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := clusterReset(cmd, args, out)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		},
	}
	return cmd
}

func clusterUpgrade(cmd *cobra.Command, args []string, out io.Writer) error {
	// TODO print what version we have and what version is available
	if !offline {
		fmt.Printf("Pulling datamesh-server docker image... ")
		resp, err := exec.Command(
			"docker", "pull", datameshDockerImage,
		).CombinedOutput()
		if err != nil {
			fmt.Printf("response: %s\n", resp)
			return err
		}
		fmt.Printf("done.\n")
	}
	fmt.Printf("Stopping datamesh-server...")
	resp, err := exec.Command(
		"docker", "rm", "-f", "datamesh-server",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("error, attempting to continue: %s\n", resp)
	} else {
		fmt.Printf("done.\n")
	}
	fmt.Printf("Stopping datamesh-server-inner...")
	resp, err = exec.Command(
		"docker", "rm", "-f", "datamesh-server-inner",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("error, attempting to continue: %s\n", resp)
	} else {
		fmt.Printf("done.\n")
	}

	pkiPath := getPkiPath()
	fmt.Printf("Starting datamesh server... ")
	err = startDatameshContainer(pkiPath)
	if err != nil {
		return err
	}
	fmt.Printf("done.\n")
	return nil
}

func clusterCommonPreflight() error {
	// - Pre-flight check, can I exec docker? Is it new enough (v1.10.0+)?
	startTiming()
	fmt.Printf("Checking suitable Docker is installed... ")
	clientVersion, err := exec.Command(
		"docker", "version", "-f", "{{.Client.Version}}",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", clientVersion)
		return err
	}
	v1_10_0, err := semver.Make("1.10.0")
	if err != nil {
		return err
	}
	cv, err := semver.Make(strings.TrimSpace(string(clientVersion)))
	if err != nil {
		fmt.Printf("assuming post-semver Docker client is sufficient.\n")
	} else {
		if cv.LT(v1_10_0) {
			return fmt.Errorf("Docker client version is < 1.10.0, please upgrade")
		}
	}

	serverVersion, err := exec.Command(
		"docker", "version", "-f", "{{.Server.Version}}",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", serverVersion)
		return err
	}
	sv, err := semver.Make(strings.TrimSpace(string(serverVersion)))
	if err != nil {
		fmt.Printf("assuming post-semver Docker server is sufficient.\n")
	} else {
		if sv.LT(v1_10_0) {
			return fmt.Errorf("Docker server version is < 1.10.0, please upgrade")
		}
		fmt.Printf("yes, got %s.\n", strings.TrimSpace(string(serverVersion)))
	}

	logTiming("check docker version")
	fmt.Printf("Checking datamesh isn't running... ")
	// - Is there a datamesh-etcd or datamesh-server container running?
	//   a) If yes, exit.
	for _, c := range []string{"datamesh-etcd", "datamesh-server"} {
		ret, err := returnCode("docker", "inspect", "--type=container", c)
		if err != nil {
			return err
		}
		if ret == 0 {
			return fmt.Errorf("%s container already exists!", c)
		}
	}
	fmt.Printf("done.\n")

	logTiming("check datamesh isn't running")
	if !offline {
		fmt.Printf("Pulling datamesh-server docker image... ")
		resp, err := exec.Command(
			"docker", "pull", datameshDockerImage,
		).CombinedOutput()
		if err != nil {
			fmt.Printf("response: %s\n", resp)
			return err
		}
		fmt.Printf("done.\n")
	}
	logTiming("pull datamesh-server docker image")
	dumpTiming()
	return nil
}

func getHostFromEnv() string {
	// use DOCKER_HOST as a hint as to where the "local" datamesh will be
	// running, from the PoV of the client
	// cases handled:
	// - DOCKER_HOST is unset: use localhost (e.g. docker on Linux, docker for
	// Mac)
	// - DOCKER_HOST=tcp://192.168.99.101:2376: parse out the bit between the
	// '://' and the second ':', because this may be a docker-machine
	// environment
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		return "127.0.0.1"
	} else {
		return strings.Split(strings.Split(dockerHost, "://")[1], ":")[0]
	}
}

func transportFromTLS(certFile, keyFile, caFile string) (*http.Transport, error) {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup HTTPS client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	return transport, nil
}

func getEtcd() (client.KeysAPI, error) {
	// attempt to connect to etcd and set the admin password for the first time
	transport, err := transportFromTLS(
		getPkiPath()+"/apiserver.pem",
		getPkiPath()+"/apiserver-key.pem",
		getPkiPath()+"/ca.pem",
	)
	if err != nil {
		return nil, err
	}
	cfg := client.Config{
		Endpoints: []string{fmt.Sprintf("https://%s:42379", getHostFromEnv())},
		Transport: transport,
		// set timeout per request to fail fast when the target endpoint is
		// unavailable
		HeaderTimeoutPerRequest: time.Second * 5,
	}
	c, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	return client.NewKeysAPI(c), nil
}

func getToken() (string, error) {
	kapi, err := getEtcd()
	if err != nil {
		return "", err
	}
	encoded, err := kapi.Get(
		context.Background(),
		fmt.Sprintf("/datamesh.io/users/%s", ADMIN_USER_UUID),
		nil,
	)
	if err != nil {
		return "", err
	}
	// just extract the field we need
	var s struct{ ApiKey string }
	err = json.Unmarshal([]byte(encoded.Node.Value), &s)
	if err != nil {
		return "", err
	}
	fmt.Printf("password: %s\n", s.ApiKey)
	return s.ApiKey, nil
}

func setTokenIfNotExists(adminPassword string) error {
	kapi, err := getEtcd()
	if err != nil {
		return err
	}
	user := struct {
		Id     string
		Name   string
		ApiKey string
	}{Id: ADMIN_USER_UUID, Name: "admin", ApiKey: adminPassword}
	encoded, err := json.Marshal(user)
	if err != nil {
		return err
	}
	_, err = kapi.Set(
		context.Background(),
		fmt.Sprintf("/datamesh.io/users/%s", ADMIN_USER_UUID),
		string(encoded),
		&client.SetOptions{PrevExist: client.PrevNoExist},
	)
	return err
}

func guessHostIPv4Addresses() ([]string, error) {
	// XXX this will break if the node's IP address changes
	ip, err := exec.Command(
		"docker", "run", "--rm", "--net=host",
		datameshDockerImage,
		"datamesh-server", "--guess-ipv4-addresses",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", ip)
		return []string{}, err
	}
	ipAddr := strings.TrimSpace(string(ip))
	return strings.Split(ipAddr, ","), nil
}

func guessHostname() (string, error) {
	hostname, err := exec.Command(
		"docker", "run", "--rm", "--net=host",
		datameshDockerImage,
		"hostname",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", hostname)
		return "", err
	}
	hostnameString := strings.TrimSpace(string(hostname))
	return hostnameString, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func startDatameshContainer(pkiPath string) error {
	if traceAddr != "" {
		fmt.Printf("Trace address: %s\n", traceAddr)
	}
	if logAddr != "" {
		fmt.Printf("Log address: %s\n", logAddr)
	}
	regStr := ""
	if allowPublicRegistrations {
		regStr = "1"
		fmt.Printf("Allowing public registration.\n")
	}
	var absoluteConfigPath string
	var configFileExists bool
	e, err := pathExists(configFile)
	if err != nil {
		fmt.Printf("we have an exists error: %s", err)
		return err
	}
	if e {
		absoluteConfigPath, err = filepath.Abs(configFile)
		if err != nil {
			return err
		}
		fmt.Printf("we have a file: %s", string(absoluteConfigPath))
		configFileExists = true
	}
	args := []string{
		"run", "--restart=always",
		"--privileged", "--pid=host", "--net=host",
		"-d", "--name=datamesh-server",
		"-v", "/lib:/system-lib/lib",
		"-v", "datamesh-kernel-modules:/bundled-lib",
		"-v", "/var/lib/docker:/var/lib/docker",
		"-v", "/run/docker:/run/docker",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		// Find bundled zfs bins and libs if exists
		"-e", "PATH=/bundled-lib/sbin:/usr/local/sbin:/usr/local/bin:" +
			"/usr/sbin:/usr/bin:/sbin:/bin",
		"-e", "LD_LIBRARY_PATH=/bundled-lib/lib:/bundled-lib/usr/lib/",
		// Allow tests to specify which pool to create and where.
		"-e", fmt.Sprintf("USE_POOL_NAME=%s", usePoolName),
		"-e", fmt.Sprintf("USE_POOL_DIR=%s", usePoolDir),
		// In case the docker daemon is older than the bundled docker client in
		// the datamesh-server image, at least allow the user to instruct it to
		// fall back to an older API version.
		"-e", fmt.Sprintf("DOCKER_API_VERSION=%s", dockerApiVersion),
		// Allow centralized tracing and logging.
		"-e", fmt.Sprintf("TRACE_ADDR=%s", traceAddr),
		"-e", fmt.Sprintf("LOG_ADDR=%s", logAddr),
		"-e", fmt.Sprintf("ALLOW_PUBLIC_REGISTRATION=%s", regStr),
		// Set env var so that sub-container executor can bind-mount the right
		// certificates in.
		"-e", fmt.Sprintf("PKI_PATH=%s", pkiPath),
		// And know their own identity, so they can respawn.
		"-e", fmt.Sprintf("DATAMESH_DOCKER_IMAGE=%s", datameshDockerImage),
		"-e", fmt.Sprintf("ASSETS_URL_PREFIX=%s", assetsURLPrefix),
		"-e", fmt.Sprintf("HOMEPAGE_URL=%s", homepageURL),
		"-e", fmt.Sprintf("FRONTEND_STATIC_FOLDER=%s", frontendStaticFolder),
	}
	if usePoolDir != "" {
		args = append(args, []string{"-v", fmt.Sprintf("%s:%s", usePoolDir, usePoolDir)}...)
	}
	if configFileExists {
		args = append(args, []string{
			"-v", fmt.Sprintf("%s:%s", absoluteConfigPath, "/etc/datamesh/config.yaml"),
		}...)
		args = append(args, []string{
			"-e", fmt.Sprintf("%s=%s", "HOST_CONFIG_PATH", absoluteConfigPath),
		}...)
	}
	args = append(args, []string{
		datameshDockerImage,
		// This attempts to download ZFS modules (if necc.) and modprobe them
		// on the Docker host before starting a second container which runs the
		// actual datamesh-server with an rshared bind-mount of /var/pool.
		"/require_zfs.sh", "datamesh-server",
	}...)
	fmt.Fprintf(logFile, "docker %s\n", strings.Join(args, " "))
	resp, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		return err
	}
	return nil
}

// exists returns whether the given file or directory exists or not
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func clusterCommonSetup(clusterUrl, adminPassword, pkiPath, clusterSecret string) error {
	// - Start etcd with discovery token on non-standard ports (to avoid
	//   conflicting with an existing etcd).
	fmt.Printf("Guessing docker host's IPv4 address (should be routable from other cluster nodes)... ")
	ipAddrs, err := guessHostIPv4Addresses()
	if err != nil {
		return err
	}
	// TODO add an argument to override this
	fmt.Printf("got: %s.\n", strings.Join(ipAddrs, ","))

	fmt.Printf("Guessing unique name for docker host (using hostname, must be unique wrt other cluster nodes)... ")
	hostnameString, err := guessHostname()
	if err != nil {
		return err
	}
	// TODO add an argument to override this
	fmt.Printf("got: %s.\n", hostnameString)

	peerURLs := []string{}
	clientURLs := []string{}
	for _, ipAddr := range ipAddrs {
		peerURLs = append(peerURLs, fmt.Sprintf("https://%s:42380", ipAddr))
		clientURLs = append(clientURLs, fmt.Sprintf("https://%s:42379", ipAddr))
	}

	fmt.Printf("Starting etcd... ")
	args := []string{
		"docker", "run", "--restart=always",
		"-d", "--name=datamesh-etcd",
		"-p", "42379:42379", "-p", "42380:42380",
		"-v", "datamesh-etcd-data:/var/lib/etcd",
		// XXX assuming you can bind-mount a path from the host is dubious,
		// hopefully it works well enough on docker for mac and docker machine.
		// An alternative approach could be to pass in the cluster secret as an
		// env var and download it and cache it in a docker volume.
		"-v", fmt.Sprintf("%s:/pki", maybeEscapeLinuxEmulatedPathOnWindows(pkiPath)),
		etcdDockerImage,
		"etcd", "--name", hostnameString,
		"--data-dir", "/var/lib/etcd",

		// Client-to-server communication:
		// Certificate used for SSL/TLS connections to etcd. When this option
		// is set, you can set advertise-client-urls using HTTPS schema.
		"--cert-file=/pki/apiserver.pem",
		// Key for the certificate. Must be unencrypted.
		"--key-file=/pki/apiserver-key.pem",
		// When this is set etcd will check all incoming HTTPS requests for a
		// client certificate signed by the trusted CA, requests that don't
		// supply a valid client certificate will fail.
		"--client-cert-auth",
		// Trusted certificate authority.
		"--trusted-ca-file=/pki/ca.pem",

		// Peer (server-to-server / cluster) communication:
		// The peer options work the same way as the client-to-server options:
		// Certificate used for SSL/TLS connections between peers. This will be
		// used both for listening on the peer address as well as sending
		// requests to other peers.
		"--peer-cert-file=/pki/apiserver.pem",
		// Key for the certificate. Must be unencrypted.
		"--peer-key-file=/pki/apiserver-key.pem",
		// When set, etcd will check all incoming peer requests from the
		// cluster for valid client certificates signed by the supplied CA.
		"--peer-client-cert-auth",
		// Trusted certificate authority.
		"--peer-trusted-ca-file=/pki/ca.pem",

		// TODO stop using the same certificate for peer and client/server
		// connection validation

		// listen
		"--listen-peer-urls", "https://0.0.0.0:42380",
		"--listen-client-urls", "https://0.0.0.0:42379",
		// advertise
		"--initial-advertise-peer-urls", strings.Join(peerURLs, ","),
		"--advertise-client-urls", strings.Join(clientURLs, ","),
	}
	if etcdInitialCluster == "" {
		args = append(args, []string{"--discovery", clusterUrl}...)
	} else {
		args = append(args, []string{
			"--initial-cluster-state", "existing",
			"--initial-cluster", etcdInitialCluster,
		}...)
	}
	resp, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		return err
	}
	fmt.Printf("done.\n")

	if adminPassword != "" {
		// we are to try and initialize the first admin password
		// try 10 times with exponentially increasing delay in between.
		// panic if the adminPassword already exists
		delay := 1
		var err error
		for i := 1; i <= 10; i++ {
			time.Sleep(time.Duration(delay) * 5 * time.Second)
			err = setTokenIfNotExists(adminPassword)
			if err == nil {
				fmt.Printf(
					"Succeeded setting initial admin password to '%s'\n",
					adminPassword,
				)
				break
			}
			delay *= 2
			fmt.Printf(
				"Can't set initial admin password yet (%s), retrying in %ds...\n",
				err, delay,
			)
		}
		if err != nil {
			return err
		}
	} else {
		// GET adminPassword from our local etcd
		// TODO refactor tryUntil
		delay := 1
		var err error
		for i := 1; i <= 10; i++ {
			time.Sleep(time.Duration(delay) * 5 * time.Second)
			adminPassword, err = getToken()
			if err == nil {
				fmt.Printf(
					"Succeeded getting initial admin password '%s'\n",
					adminPassword,
				)
				break
			}
			delay *= 2
			fmt.Printf(
				"Can't get initial admin password yet (%s), retrying in %ds...\n",
				err, delay,
			)
		}
		if err != nil {
			return err
		}
	}
	// set the admin password in our Configuration
	fmt.Printf("Configuring dm CLI to authenticate to datamesh server %s... ", configPath)
	config, err := remotes.NewConfiguration(configPath)
	if err != nil {
		return err
	}
	err = config.AddRemote("local", "admin", getHostFromEnv(), adminPassword)
	if err != nil {
		return err
	}
	err = config.SetCurrentRemote("local")
	if err != nil {
		return err
	}
	fmt.Printf("done.\n")

	// - Start datamesh-server.
	fmt.Printf("Starting datamesh server... ")
	err = startDatameshContainer(pkiPath)
	if err != nil {
		return err
	}
	fmt.Printf("done.\n")
	fmt.Printf("Waiting for datamesh server to come up")
	connected := false
	try := 0
	for !connected {
		try++
		connected = func() bool {
			dm, err := remotes.NewDatameshAPI(configPath)
			e := func() {
				if try == 4*30 { // 30 seconds (250ms per try)
					fmt.Printf(
						"\nUnable to connect to datamesh server after 30s, " +
							"please run `docker logs datamesh-server` " +
							"and paste the result into an issue at " +
							"https://github.com/datamesh-io/datamesh/issues/new\n")
				}
				fmt.Printf(".")
				time.Sleep(250 * time.Millisecond)
			}
			if err != nil {
				e()
				return false
			}
			var response bool
			response, err = dm.Ping()
			if err != nil {
				e()
				return false
			}
			if !response {
				e()
			}
			fmt.Printf("\n")
			return response
		}()
	}
	fmt.Printf("done.\n")
	return nil
}

func clusterReset(cmd *cobra.Command, args []string, out io.Writer) error {
	// TODO this should gather a _list_ of errors, not just at-most-one!
	var bailErr error

	fmt.Printf("Destroying all datamesh data... ")
	resp, err := exec.Command(
		"docker", "exec", "datamesh-server-inner",
		"zfs", "destroy", "-r", "pool",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		bailErr = err
	}
	fmt.Printf("done.\n")

	fmt.Printf("Deleting datamesh-etcd container... ")
	resp, err = exec.Command(
		"docker", "rm", "-v", "-f", "datamesh-etcd",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		bailErr = err
	}
	fmt.Printf("done.\n")
	fmt.Printf("Deleting datamesh-server containers... ")
	resp, err = exec.Command(
		"docker", "rm", "-v", "-f", "datamesh-server",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		bailErr = err
	}
	fmt.Printf("done.\n")
	fmt.Printf("Deleting datamesh-server-inner containers... ")
	resp, err = exec.Command(
		"docker", "rm", "-v", "-f", "datamesh-server-inner",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		bailErr = err
	}
	fmt.Printf("done.\n")

	// - Delete datamesh socket
	fmt.Printf("Deleting datamesh socket... ")
	resp, err = exec.Command(
		"rm", "-f", "/run/docker/plugins/dm.sock",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		bailErr = err
	}
	fmt.Printf("done.\n")

	fmt.Printf("Deleting datamesh-etcd-data local volume... ")
	resp, err = exec.Command(
		"docker", "volume", "rm", "datamesh-etcd-data",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		bailErr = err
	}
	fmt.Printf("done.\n")
	fmt.Printf("Deleting datamesh-kernel-modules local volume... ")
	resp, err = exec.Command(
		"docker", "volume", "rm", "datamesh-kernel-modules",
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		bailErr = err
	}
	fmt.Printf("done.\n")
	fmt.Printf("Deleting 'local' remote... ")
	config, err := remotes.NewConfiguration(configPath)
	if err != nil {
		fmt.Printf("response: %s\n", resp)
		bailErr = err
	} else {
		err = config.RemoveRemote("local")
		if err != nil {
			fmt.Printf("response: %s\n", resp)
			bailErr = err
		}
	}
	fmt.Printf("done.\n")
	fmt.Printf("Deleting cached PKI assets... ")
	pkiPath := getPkiPath()
	clientVersion, err := exec.Command(
		"rm", "-rf", pkiPath,
	).CombinedOutput()
	if err != nil {
		fmt.Printf("response: %s\n", clientVersion)
		bailErr = err
	}
	fmt.Printf("done.\n")
	if bailErr != nil {
		return bailErr
	}
	return nil
}

func generatePkiJsonEncoded(pkiPath string) (string, error) {
	v := url.Values{}
	resultMap := map[string]string{}
	files, err := ioutil.ReadDir(pkiPath)
	if err != nil {
		return "", err
	}
	for _, file := range files {
		name := file.Name()
		c, err := ioutil.ReadFile(pkiPath + "/" + name)
		if err != nil {
			return "", err
		}
		resultMap[name] = string(c)
	}
	j, err := json.Marshal(resultMap)
	if err != nil {
		return "", err
	}
	v.Set("value", string(j))
	return v.Encode(), nil
}

func getPkiPath() string {
	dirPath := filepath.Dir(configPath)
	pkiPath := dirPath + "/pki"
	return pkiPath
}

func maybeEscapeLinuxEmulatedPathOnWindows(path string) string {
	// If the 'dm' client is running on Windows in WSL (Windows Subsystem for
	// Linux), and the Linux docker client is installed in the WSL environment,
	// and Docker for Windows is installed, we need to escape the WSL chroot
	// path, before being passed to Docker as a Windows path. E.g.
	//
	// /home/$USER/.datamesh/pki
	//   ->
	// C:/Users/$USER/AppData/Local/lxss/home/$USER/.datamesh/pki
	//
	// We can determine whether this is necessary by reading /proc/version
	// https://github.com/Microsoft/BashOnWindows/issues/423#issuecomment-221627364

	version, err := os.ReadFile("/proc/version")
	if err != nil {
		panic(err)
	}
	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	if strings.Contains(version, "Microsoft") {
		// In test environment, user was 'User' and Linux user was 'user'.
		// Hopefully lowercasing is the only transformation.  Hopefully on the
		// Windows (docker server) side, the path is case insensitive!
		return "C:/Users/" + user.Username + "/AppData/Local/lxss" + path
	}
	return path
}

func generatePKI(extantCA bool) error {
	// TODO add generatePKI(true) after getting PKI material from discovery
	// server
	ipAddrs, err := guessHostIPv4Addresses()
	if err != nil {
		return err
	}
	hostname, err := guessHostname()
	if err != nil {
		return err
	}
	advertise := []string{getHostFromEnv()}
	advertise = append(advertise, ipAddrs...)
	pkiPath := getPkiPath()
	_, _, err = pki.CreatePKIAssets(pkiPath, &pki.Configuration{
		AdvertiseAddresses: advertise,
		ExternalDNSNames:   []string{"datamesh-etcd", "localhost", hostname}, // TODO allow arg
		ExtantCA:           extantCA,
	})
	if err != nil {
		return err
	}
	return nil
}

func clusterInit(cmd *cobra.Command, args []string, out io.Writer) error {
	// - Run clusterCommonPreflight.
	err := clusterCommonPreflight()
	if err != nil {
		return err
	}
	fmt.Printf("Registering new cluster... ")
	// - Get a unique cluster id by asking discovery.datamesh.io.
	// support specifying size here (to avoid cliques/enable HA)
	resp, err := http.Get(fmt.Sprintf("%s/new?size=%d", discoveryUrl, serverCount))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Printf("got URL:\n%s\n", string(body))

	// - Generate PKI material, and upload it to etcd at hidden clusterSecret
	fmt.Printf("Generating PKI assets... ")
	clusterSecret, err := RandToken(32)
	if err != nil {
		return err
	}
	pkiPath := getPkiPath()
	_, err = os.Stat(pkiPath)
	if err == nil {
		return fmt.Errorf(
			"PKI directory already exists at %s, refusing to proceed", pkiPath,
		)
	}
	if !os.IsNotExist(err) {
		return err
	}

	err = os.Mkdir(pkiPath, 0700)
	if err != nil {
		return err
	}
	err = generatePKI(false)
	fmt.Printf("done.\n")

	// - Upload all PKI assets to discovery.datamesh.io under "secure" path
	pkiJsonEncoded, err := generatePkiJsonEncoded(pkiPath)
	if err != nil {
		return err
	}
	putPath := fmt.Sprintf("%s/_secrets/_%s", string(body), clusterSecret)

	req, err := http.NewRequest("PUT", putPath, bytes.NewBufferString(pkiJsonEncoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	//fmt.Printf("Response: %s\n", r)

	// - Generate admin password, and insert it into etcd
	adminPassword, err := RandToken(32)
	if err != nil {
		return err
	}

	// If you specified --count > 1, you'll need to do this immediately.
	fmt.Printf(
		"If you want more than one node in your cluster, run this on other nodes:\n\n"+
			"    dm cluster join %s:%s\n\n"+
			"This is the last time this secret will be printed, so keep it safe!\n\n",
		string(body), clusterSecret,
	)
	if serverCount > 1 {
		fmt.Printf("=====================================================================\n" +
			"You specified --count > 1, you'll need to run this on the other nodes\n" +
			"immediately, before the following setup will complete.\n" +
			"=====================================================================\n",
		)
	}

	// - Run clusterCommonSetup.
	err = clusterCommonSetup(
		strings.TrimSpace(string(body)), adminPassword, pkiPath, clusterSecret,
	)
	if err != nil {
		return err
	}
	return nil
}

func clusterJoin(cmd *cobra.Command, args []string, out io.Writer) error {
	// - Run clusterCommonPreflight.
	err := clusterCommonPreflight()
	if err != nil {
		return err
	}
	// - Require unique cluster id and secret to be specified.
	if len(args) != 1 {
		return fmt.Errorf("Please specify <cluster-url>:<secret> as argument.")
	}
	// - Download PKI assets
	fmt.Printf("Downloading PKI assets... ")
	shrapnel := strings.Split(args[0], ":")
	clusterUrlPieces := []string{}
	// construct the 'https://host:port/path' (clusterUrl) from
	// 'https://host:port/path:secret' and save 'secret' to clusterSecret
	for i := 0; i < len(shrapnel)-1; i++ {
		clusterUrlPieces = append(clusterUrlPieces, shrapnel[i])
	}
	clusterUrl := strings.Join(clusterUrlPieces, ":")
	clusterSecret := shrapnel[len(shrapnel)-1]

	adminPassword := "" // aka "don't attempt to set it in etcd"
	pkiPath := getPkiPath()
	// Now get PKI assets from discovery service.
	// TODO: discovery service should mint new credentials just for us, rather
	// than handing us the keys to the kingdom.
	// https://github.com/datamesh-io/datamesh/issues/21
	//fmt.Printf("clusterUrl: %s\n", clusterUrl)
	getPath := fmt.Sprintf("%s/_secrets/_%s", clusterUrl, clusterSecret)
	//fmt.Printf("getPath: %s\n", getPath)
	resp, err := http.Get(getPath)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	//fmt.Printf(string(body))
	var etcdNode map[string]interface{}
	err = json.Unmarshal(body, &etcdNode)
	if err != nil {
		return err
	}
	var filesContents map[string]string
	err = json.Unmarshal(
		[]byte(
			(etcdNode["node"].(map[string]interface{}))["value"].(string),
		), &filesContents,
	)
	//fmt.Printf("===\nfilesContents is %s\n===\n", filesContents)
	// first check whether the directory exists
	if _, err := os.Stat(pkiPath); err == nil {
		return fmt.Errorf(
			"PKI already exists at %s, refusing to proceed. Run 'sudo dm cluster reset' to clean up.",
			pkiPath,
		)
	}
	if _, err := os.Stat(pkiPath + ".tmp"); err == nil {
		return fmt.Errorf(
			"PKI already exists at %s, refusing to proceed. "+
				"Delete the stray temporary directory and run 'sudo dm cluster reset' to try again.",
			pkiPath+".tmp",
		)
	}

	err = os.MkdirAll(pkiPath+".tmp", 0700)
	if err != nil {
		return err
	}
	for filename, contents := range filesContents {
		err = ioutil.WriteFile(pkiPath+".tmp/"+filename, []byte(contents), 0600)
		if err != nil {
			return err
		}
	}
	err = os.Rename(pkiPath+".tmp", pkiPath)
	if err != nil {
		return err
	}
	err = generatePKI(true)
	if err != nil {
		return err
	}
	fmt.Printf("done!\n")
	// - Run clusterCommonSetup.
	err = clusterCommonSetup(clusterUrl, adminPassword, pkiPath, clusterSecret)
	if err != nil {
		return err
	}
	return nil
}

func returnCode(name string, arg ...string) (int, error) {
	// Run a command and either get the returncode or an error if the command
	// failed to execute, based on
	// http://stackoverflow.com/questions/10385551/get-exit-code-go
	cmd := exec.Command(name, arg...)
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus(), nil
			}
		} else {
			return -1, err
		}
	}
	// got here, so err == nil
	return 0, nil
}
