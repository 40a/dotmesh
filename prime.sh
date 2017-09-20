#!/usr/bin/env bash
set -xe
cd ~/kubernetes
#
# sudo :-(
# otherwise we get a bunch of stuff like:
#
#     chown: changing ownership of '/home/luke/kubernetes/_output/images/kube-build:build-59cac78cf2-5-v1.8.3-2/Dockerfile': Operation not permitted
#
sudo EXTRA_DOCKER_ARGS= dind/dind-cluster.sh bare prime-images
docker rm -f prime-images

cd $GOPATH/src/github.com/lukemarsden/datamesh/cmd/datamesh-server
./rebuild.sh
./prime-docker.sh
