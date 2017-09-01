#!/usr/bin/env bash
set +xe
cd ~/kubernetes
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

cd ~/datamesh-instrumentation/etcd-browser
docker build -t $(hostname).local:80/lukemarsden/etcd-browser:v1 .
docker push $(hostname).local:80/lukemarsden/etcd-browser:v1
