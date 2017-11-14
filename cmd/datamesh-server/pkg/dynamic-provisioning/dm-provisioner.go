/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/rpc/v2/json2"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/external-storage/lib/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"syscall"
)

const (
	resyncPeriod              = 15 * time.Second
	provisionerName           = "datamesh/datamesh-dynamic-provisioner"
	exponentialBackOffOnError = false
	failedRetryThreshold      = 5
	leasePeriod               = controller.DefaultLeaseDuration
	retryPeriod               = controller.DefaultRetryPeriod
	renewDeadline             = controller.DefaultRenewDeadline
	termLimit                 = controller.DefaultTermLimit
)

type datameshProvisioner struct {
}

// NewDatameshProvisioner creates a new datamesh provisioner
func NewDatameshProvisioner() controller.Provisioner {
	return &datameshProvisioner{}
}

var _ controller.Provisioner = &datameshProvisioner{}

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

// Provision creates a storage asset and returns a PV object representing it.
func (p *datameshProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	// PV name: options.PVName
	// options is a https://godoc.org/github.com/kubernetes-incubator/external-storage/lib/controller#VolumeOptions
	// options.PVC is a https://godoc.org/k8s.io/kubernetes/pkg/api#PersistentVolumeClaim
	// options.Parameters = the storage class parameters

	// options.PVC.ObjectMeta.Annotations = the PVC annotations

	// Read API key from environment

	apiKey := os.Getenv("DATAMESH_API_KEY")
	fmt.Printf("API KEY: '%s'\n", apiKey)

	// Read storage class options
	datameshNode, ok := options.Parameters["datameshNode"]
	if !ok {
		datameshNode = "127.0.0.1"
	}

	user, ok := options.Parameters["datameshUser"]
	if !ok {
		user = "admin"
	}

	namespace, ok := options.Parameters["datameshNamespace"]
	if !ok {
		namespace = user
	}

	// Read PVC annotations, which can override some options

	annotations := options.PVC.ObjectMeta.Annotations

	pvcNamespace, ok := annotations["datameshNamespace"]
	if ok {
		namespace = pvcNamespace
	}

	name, ok := annotations["datameshVolume"]
	if !ok {
		// No name specified? Default to PVC name.
		name = options.PVC.ObjectMeta.Name
	}

	var mountPath string

	err := doRPC(
		datameshNode,
		user,
		apiKey,
		"DatameshRPC.Procure",
		map[string]string{
			"Name":      name,
			"Namespace": namespace,
		},
		&mountPath,
	)
	if err != nil {
		return nil, err
	}

	// FIXME: Now unmount the mountPath, we don't need it yet

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
			Annotations: map[string]string{
				// Record some stuff we have, in case it's useful for debugging or anything
				// But not the API key, obviously.
				"datameshNode":      datameshNode,
				"datameshUser":      user,
				"datameshNamespace": namespace,
				"datameshVolume":    name,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			// This big struct is documented here:
			// https://godoc.org/k8s.io/kubernetes/pkg/api#PersistentVolumeSource
			PersistentVolumeSource: v1.PersistentVolumeSource{
				FlexVolume: &v1.FlexVolumeSource{
					Driver: "datamesh.io/dm",
					FSType: "zfs",
					Options: map[string]string{
						"name":      name,
						"namespace": namespace,
					},
				},
			},
		},
	}

	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *datameshProvisioner) Delete(volume *v1.PersistentVolume) error {
	/*
		      volume.Annotations["datameshProvisionerNamespace"]
		      volume.Annotations["datameshProvisionerVolume"]
				ann, ok := volume.Annotations["datameshProvisionerIdentity"]
				if !ok {
					return errors.New("identity annotation not found on PV")
				}

		               Delete DM volume?
					      Or do nothing as we just "detach"?
					      Look up the actual use case here.
	*/

	return nil
}

func main() {
	syscall.Umask(0)

	flag.Parse()
	flag.Set("logtostderr", "true")

	glog.Info("Starting")

	// Create an InClusterConfig and use it to create a client for the controller
	// to use to communicate with Kubernetes
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatalf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		glog.Fatalf("Error getting server version: %v", err)
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	datameshProvisioner := NewDatameshProvisioner()

	// Start the provision controller which will dynamically provision datamesh
	// PVs
	pc := controller.NewProvisionController(clientset, resyncPeriod, provisionerName, datameshProvisioner, serverVersion.GitVersion, exponentialBackOffOnError, failedRetryThreshold, leasePeriod, renewDeadline, retryPeriod, termLimit)
	pc.Run(wait.NeverStop)
}
