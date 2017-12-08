# Datamesh on Kubernetes

Datamesh supports:

* being deployed on Kubernetes
* providing persistent volumes to Kubernetes pods

## Prerequisites

You need a Kubernetes 1.7.0+ cluster with working `hostPort` support.

If you are using a `kubeadm` cluster with Weave Net, following the instructions [here](https://github.com/weaveworks/weave/issues/3016#issuecomment-321337923) on each of your nodes may help.

## Getting started

Get started by creating an initial admin API key, then deploy Datamesh with:

```
kubectl create namespace datamesh
echo "secret123" > datamesh-admin-password.txt
kubectl create secret generic datamesh --from-file=datamesh-admin-password.txt -n datamesh
rm datamesh-admin-password.txt
kubectl apply -f manifests/
```

Then load http://`<address-of-cluster-nodes>`:6969/ux in your browser and log in with username `admin` and the password you specified (`secret123` in the example above) to see the Datamesh UI.

Now you can use your Kubernetes cluster as a Datamesh remote!

```
sudo curl -o /usr/local/bin/dm https://get.datamesh.io/$(uname -s)/dm
sudo chmod +x /usr/local/bin/dm
dm remote add kube admin@<address-of-cluster-nodes>
dm list
```

Enter the admin password you specified (`secret123` in the example above), then you should be able to list Kubernetes-provisioned volumes with `dm list` and push/pull volumes between clusters with `dm push`, etc.

TODO: StorageClass example using Datamesh for dynamic provisioning (how to get a volume in the first place).

TODO: a TPR for datamesh volumes to experiment with fancy stuff?
Examples of declarative config for e.g. regular backups?
Federation API server volume implementation?


## Notes from installing on GKE / AWS

gke instances don't let you create a `/datamesh-test-pools` folder on the root because `/` is read-only
TODO: allow the path of the `/datamesh-test-pools` to be configurable.

gke cluster must be run in alpha to enable the `rbac.authorization.k8s.io/v1alpha1` apiVersion

aws cluster uses `4.4.102-k8s` kernel version and we got a 404 on `https://get.datamesh.io/zfs/zfs-4.4.102-k8s.tar.gz`

## Installing on gcloud


NOTE: on each node you need to:

```bash
sudo mkdir -p /datamesh-test-pools
sudo mount --bind /datamesh-test-pools /datamesh-test-pools
sudo mount --make-shared /datamesh-test-pools
```

```bash
gcloud alpha container clusters create testdm --enable-kubernetes-alpha
gcloud container clusters get-credentials testdm
kubectl create namespace datamesh
echo "secret123" > datamesh-admin-password.txt
kubectl create secret generic datamesh --from-file=datamesh-admin-password.txt -n datamesh
rm datamesh-admin-password.txt
cat etcd-operator-dep.yaml | kubectl apply -f -
cat datamesh.yaml | kubectl apply -f -
```
