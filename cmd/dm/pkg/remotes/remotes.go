package remotes

/*
A local configuration system for storing "clusters" (remote API targets for the
CLI) and authentication tokens for accessing them.

A user can log in to zero or more clusters. It's important that they're able to
be logged in to more than one cluster at a time, for example to be able to
"push" from one to another.

$ dm remote add origin luke@192.168.1.12
Logging in to datamesh cluster at 192.168.1.12...
API key: deadbeefcafebabe
Checking login... confirmed!
Login saved in local configuration. Active cluster now origin.

$ dm remote -v
origin     luke@192.168.1.12

How this diverges from git: the CLI itself is logged into one global set of
"remotes", *not* per-repo. This is because there are no local repos. Does this
matter?
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"

	"golang.org/x/net/context"

	"github.com/gorilla/rpc/v2/json2"
	"github.com/opentracing/opentracing-go"
	opentracinglog "github.com/opentracing/opentracing-go/log"
	"github.com/openzipkin/zipkin-go-opentracing/examples/middleware"
)

type Remote struct {
	User                 string
	Hostname             string
	ApiKey               string
	CurrentVolume        string
	CurrentBranches      map[string]string
	DefaultRemoteVolumes map[string]map[string]VolumeName
}

type Configuration struct {
	CurrentRemote string
	Remotes       map[string]*Remote
	lock          sync.Mutex
	configPath    string
}

func NewConfiguration(configPath string) (*Configuration, error) {
	c := &Configuration{
		configPath: configPath,
		Remotes:    make(map[string]*Remote),
	}
	if err := c.Load(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Configuration) Load() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, err := os.Stat(c.configPath); os.IsNotExist(err) {
		// Just return with defaults if file does not exist
		return nil
	}
	serialized, err := ioutil.ReadFile(c.configPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(serialized, &c); err != nil {
		return err
	}
	return nil
}

func (c *Configuration) Save() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.save()
}

func (c *Configuration) save() error {
	serialized, err := json.Marshal(c)
	if err != nil {
		return err
	}
	ioutil.WriteFile(c.configPath, serialized, 0600)
	return nil
}

func (c *Configuration) GetRemote(name string) (*Remote, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	r, ok := c.Remotes[name]
	if !ok {
		return nil, fmt.Errorf("Unable to find remote '%s'", name)
	}
	return r, nil
}

func (c *Configuration) GetRemotes() map[string]*Remote {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.Remotes
}

func (c *Configuration) GetCurrentRemote() string {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.CurrentRemote
}

func (c *Configuration) SetCurrentRemote(remote string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, ok := c.Remotes[remote]
	if !ok {
		return fmt.Errorf("No such remote '%s'", remote)
	}
	c.CurrentRemote = remote
	return c.save()
}

func (c *Configuration) CurrentVolume() (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.currentVolume()
}

func (c *Configuration) currentVolume() (string, error) {
	r, ok := c.Remotes[c.CurrentRemote]
	if !ok {
		return "", fmt.Errorf("No current volume. List volumes with 'dm list' and switch to one with 'dm switch'.")
	}
	return r.CurrentVolume, nil
}

func (c *Configuration) SetCurrentVolume(volume string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, ok := c.Remotes[c.CurrentRemote]
	if !ok {
		return fmt.Errorf(
			"Unable to find remote '%s', which was apparently current",
			c.CurrentRemote,
		)
	}
	(*c.Remotes[c.CurrentRemote]).CurrentVolume = volume
	return c.save()
}

func (c *Configuration) DefaultRemoteVolumeFor(peer, namespace, volume string) (string, string, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	defaultRemoteVolume, ok := c.Remotes[peer].DefaultRemoteVolumes[namespace][volume]
	if !ok {
		return "", "", false
	}
	return defaultRemoteVolume.Namespace, defaultRemoteVolume.Name, true
}

func (c *Configuration) SetDefaultRemoteVolumeFor(peer, namespace, volume, remoteNamespace, remoteVolume string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, ok := c.Remotes[peer]
	if !ok {
		return fmt.Errorf(
			"Unable to find remote '%s', which was apparently current",
			c.CurrentRemote,
		)
	}
	if c.Remotes[c.CurrentRemote].DefaultRemoteVolumes == nil {
		c.Remotes[c.CurrentRemote].DefaultRemoteVolumes = map[string]map[string]VolumeName{}
	}
	if c.Remotes[c.CurrentRemote].DefaultRemoteVolumes[namespace] == nil {
		c.Remotes[c.CurrentRemote].DefaultRemoteVolumes[namespace] = map[string]VolumeName{}
	}
	c.Remotes[c.CurrentRemote].DefaultRemoteVolumes[namespace][volume] = VolumeName{remoteNamespace, remoteVolume}
	return c.save()
}

func (c *Configuration) CurrentBranchFor(volume string) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	currentBranch, ok := c.Remotes[c.CurrentRemote].CurrentBranches[volume]
	if !ok {
		return DEFAULT_BRANCH, nil
	}
	return currentBranch, nil
}

func (c *Configuration) CurrentBranch() (string, error) {
	c.lock.Lock()
	cur, err := c.currentVolume()
	c.lock.Unlock()
	if err != nil {
		return "", err
	}
	return c.CurrentBranchFor(cur)
}

func (c *Configuration) SetCurrentBranch(branch string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	cur, err := c.currentVolume()
	if err != nil {
		return err
	}
	c.Remotes[c.CurrentRemote].CurrentBranches[cur] = branch
	return c.save()
}

func (c *Configuration) SetCurrentBranchForVolume(volume, branch string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, ok := c.Remotes[c.CurrentRemote]
	if !ok {
		return fmt.Errorf(
			"Unable to find remote '%s', which was apparently current",
			c.CurrentRemote,
		)
	}
	if c.Remotes[c.CurrentRemote].CurrentBranches == nil {
		c.Remotes[c.CurrentRemote].CurrentBranches = map[string]string{}
	}
	c.Remotes[c.CurrentRemote].CurrentBranches[volume] = branch
	return c.save()
}

func (c *Configuration) AddRemote(remote, user, hostname, apiKey string) error {
	_, ok := c.Remotes[remote]
	if ok {
		return fmt.Errorf("Remote exists '%s'", remote)
	}
	c.Remotes[remote] = &Remote{
		User:     user,
		Hostname: hostname,
		ApiKey:   apiKey,
	}
	return c.save()
}

func (c *Configuration) RemoveRemote(remote string) error {
	_, ok := c.Remotes[remote]
	if !ok {
		return fmt.Errorf("No such remote '%s'", remote)
	}
	delete(c.Remotes, remote)
	if c.CurrentRemote == remote {
		c.CurrentRemote = ""
	}
	return c.save()
}

func (c *Configuration) ClusterFromRemote(remote string) (*JsonRpcClient, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	remoteCreds, ok := c.Remotes[remote]
	if !ok {
		return nil, fmt.Errorf("No such remote '%s'", remote)
	}
	return &JsonRpcClient{
		User:     remoteCreds.User,
		Hostname: remoteCreds.Hostname,
		ApiKey:   remoteCreds.ApiKey,
	}, nil
}

func (c *Configuration) ClusterFromCurrentRemote() (*JsonRpcClient, error) {
	return c.ClusterFromRemote(c.CurrentRemote)
}

type JsonRpcClient struct {
	User     string
	Hostname string
	ApiKey   string
}

// call a method with string args, and attempt to decode it into result
func (j *JsonRpcClient) CallRemote(
	ctx context.Context, method string, args interface{}, result interface{},
) error {
	// create new span using span found in context as parent (if none is found,
	// our span becomes the trace root).
	span, ctx := opentracing.StartSpanFromContext(ctx, method)
	span.LogFields(
		opentracinglog.String("type", "cli-rpc"),
		opentracinglog.String("method", method),
		opentracinglog.String("args", fmt.Sprintf("%v", args)),
	)
	defer span.Finish()

	if j == nil {
		return fmt.Errorf(
			"No remote cluster specified. List remotes with 'dm remote -v'. " +
				"Choose one with 'dm remote switch' or create one with 'dm remote " +
				"add'. Try 'dm cluster init' if you don't have a cluster yet.",
		)
	}
	url := fmt.Sprintf("http://%s:6969/rpc", j.Hostname)
	message, err := json2.EncodeClientRequest(method, args)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(message))
	if err != nil {
		return err
	}

	tracer := opentracing.GlobalTracer()
	// use our middleware to propagate our trace
	req = middleware.ToHTTPRequest(tracer)(req.WithContext(ctx))

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(j.User, j.ApiKey)
	client := new(http.Client)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		// TODO add user mgmt subcommands, then reference them in this error message
		// annotate our span with the error condition
		span.SetTag("error", "Permission denied")
		return fmt.Errorf("Permission denied. Please check that your API key is still valid.")
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		span.SetTag("error", err.Error())
		return fmt.Errorf("Error reading body: %s", err)
	}
	err = json2.DecodeClientResponse(bytes.NewBuffer(b), &result)
	if err != nil {
		span.SetTag("error", fmt.Sprintf("Response '%s' yields error %s", string(b), err))
		return fmt.Errorf("Response '%s' yields error %s", string(b), err)
	}
	return nil
}
