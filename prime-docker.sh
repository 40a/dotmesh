#!/usr/bin/env bash
set -xe

cd $GOPATH/src/github.com/datamesh-io/datamesh/cmd/datamesh-server
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
