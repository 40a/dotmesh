/*
The datamesh server.

Essentially an etcd-clustered set of state machines which correspond to ZFS
filesystems.
*/

package main

import (
	"fmt"
	"io/ioutil"
	"log" // TODO start using https://github.com/Sirupsen/logrus
	"os"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	"gopkg.in/yaml.v2"
)

// initial setup
const ROOT_FS = "dmfs"
const ZFS = "zfs"
const ZPOOL = "zpool"
const META_KEY_PREFIX = "io.datamesh:meta-"
const ETCD_PREFIX = "/datamesh.io"
const CONTAINER_MOUNT_PREFIX = "/var/datamesh"
const CONFIG_PATH = "/etc/datamesh/config.yml"

var LOG_TO_STDOUT bool
var POOL string

func main() {

	var config Config
	var foundConfig bool

	dat, err := ioutil.ReadFile(CONFIG_PATH)

	if os.IsNotExist(err) {
		config = Config{}
		foundConfig = false
	} else {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err := yaml.Unmarshal(dat, &config)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		foundConfig = true
	}

	POOL = os.Getenv("POOL")
	if POOL == "" {
		POOL = "pool"
	}
	traceAddr := os.Getenv("TRACE_ADDR")
	if traceAddr != "" {
		collector, err := zipkin.NewHTTPCollector(
			fmt.Sprintf("http://%s:9411/api/v1/spans", traceAddr),
		)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		tracer, err := zipkin.NewTracer(
			zipkin.NewRecorder(collector, false, "127.0.0.1:0", "datamesh"),
			zipkin.ClientServerSameSpan(true),
			zipkin.TraceID128Bit(true),
		)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		opentracing.InitGlobalTracer(tracer)
	}
	// TODO proper flag parsing
	if len(os.Args) > 1 && os.Args[1] == "--guess-ipv4-addresses" {
		addresses, err := guessIPv4Addresses()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(strings.Join(addresses, ","))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--temporary-error-plugin" {
		s := NewInMemoryState("<unknown>", config)
		s.runErrorPlugin()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		LOG_TO_STDOUT = false
	} else {
		LOG_TO_STDOUT = true
	}
	setupLogging()
	log.Print("Oh, hello.")
	if foundConfig {
		log.Print("Successfully loaded config from %s.", CONFIG_PATH)
	} else {
		log.Print("No config found at %s, using defaults.", CONFIG_PATH)
	}

	localPoolId, err := findLocalPoolId()
	if err != nil {
		out("Unable to determine pool ID. Make sure to run me as root.\n" +
			"Please create a ZFS pool called '" + POOL + "'.\n" +
			"The following commands will create a toy pool-in-a-file:\n\n" +
			"    sudo truncate -s 10G /pool-datafile\n" +
			"    sudo zpool create pool /pool-datafile\n\n" +
			"Otherwise, see 'man zpool' for how to create a real pool.\n" +
			"If you don't have the 'zpool' tool installed, on Ubuntu 16.04, run:\n\n" +
			"    sudo apt-get install zfsutils-linux\n\n" +
			"On other distributions, follow the instructions at http://zfsonlinux.org/\n")
		log.Fatalf("Unable to find pool ID, I don't know who I am :( %s %s", err, localPoolId)
	}
	ips, _ := guessIPv4Addresses()
	log.Printf("Detected my node ID as %s (%s)", localPoolId, ips)
	s := NewInMemoryState(localPoolId, config)

	for _, filesystemId := range findFilesystemIdsOnSystem() {
		log.Printf("Initializing fsMachine for %s", filesystemId)
		s.initFilesystemMachine(filesystemId)
	}
	go runForever(
		s.updateAddressesInEtcd, "updateAddressesInEtcd",
		// ttl on address keys will be 60 seconds, so update them every 30
		// (hopefully updating them doesn't take >30 seconds)
		1*time.Second, 30*time.Second,
	)
	// kick off an on-startup perusal of which dm containers are running
	go runForever(s.fetchRelatedContainers, "fetchRelatedContainers",
		1*time.Second, 1*time.Second,
	)
	// TODO proper flag parsing
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		go runForever(s.fetchAndWatchEtcd, "fetchAndWatchEtcd",
			1*time.Second, 1*time.Second,
		)
		s.repl()
	} else {
		runForever(s.fetchAndWatchEtcd, "fetchAndWatchEtcd",
			1*time.Second, 1*time.Second,
		)
	}
}
