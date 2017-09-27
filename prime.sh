#!/usr/bin/env bash
set -xe

cd $GOPATH/src/github.com/datamesh-io/datamesh/cmd/datamesh-server
./rebuild.sh
docker build -t $(hostname).local:80/datamesh/datamesh-server:latest .
docker push $(hostname).local:80/datamesh/datamesh-server:latest

docker pull quay.io/datamesh/etcd:v3.0.15
docker tag quay.io/datamesh/etcd:v3.0.15 $(hostname).local:80/datamesh/etcd:v3.0.15
docker push $(hostname).local:80/datamesh/etcd:v3.0.15

docker pull busybox
docker tag busybox $(hostname).local:80/busybox
docker push $(hostname).local:80/busybox

docker pull mysql:5.7.17
docker tag mysql:5.7.17 $(hostname).local:80/mysql:5.7.17
docker push $(hostname).local:80/mysql:5.7.17

cd ~/datamesh-instrumentation/etcd-browser
docker build -t $(hostname).local:80/datamesh/etcd-browser:v1 .
docker push $(hostname).local:80/datamesh/etcd-browser:v1

# Cache images required by Kubernetes

declare -A cache

cache["gcr.io/google_containers/etcd-amd64:3.0.17"]="google_containers/etcd-amd64:3.0.17"
cache["gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.4"]="google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.4"
cache["gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.4"]="google_containers/k8s-dns-kube-dns-amd64:1.14.4"
cache["gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.4"]="google_containers/k8s-dns-sidecar-amd64:1.14.4"
cache["gcr.io/google_containers/kube-apiserver-amd64:v1.7.6"]="google_containers/kube-apiserver-amd64:v1.7.6"
cache["gcr.io/google_containers/kube-controller-manager-amd64:v1.7.6"]="google_containers/kube-controller-manager-amd64:v1.7.6"
cache["gcr.io/google_containers/kube-proxy-amd64:v1.7.6"]="google_containers/kube-proxy-amd64:v1.7.6"
cache["gcr.io/google_containers/kube-scheduler-amd64:v1.7.6"]="google_containers/kube-scheduler-amd64:v1.7.6"
cache["quay.io/coreos/etcd-operator:v0.5.0"]="coreos/etcd-operator:v0.5.0"
cache["quay.io/datamesh/datamesh-server:latest"]="datamesh/datamesh-server:latest"
cache["weaveworks/weave-kube:2.0.4"]="weaveworks/weave-kube:2.0.4"
cache["weaveworks/weave-npc:2.0.4"]="weaveworks/weave-npc:2.0.4"
cache["gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.4"]="google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.4"
cache["gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.4"]="google_containers/k8s-dns-kube-dns-amd64:1.14.4"
cache["gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.4"]="google_containers/k8s-dns-sidecar-amd64:1.14.4"
cache["quay.io/coreos/etcd-operator:v0.5.0"]="coreos/etcd-operator:v0.5.0"
cache["quay.io/datamesh/datamesh-server:latest"]="datamesh/datamesh-server:latest"

for fq_image in "${!cache[@]}"; do
    local_name="$(hostname).local:80/${cache[$fq_image]}"
    docker pull $fq_image
    docker tag $fq_image $local_name
    docker push $local_name
done
