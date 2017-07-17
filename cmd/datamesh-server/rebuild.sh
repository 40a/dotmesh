#!/bin/bash
set -xe
mkdir -p target
docker build -f Dockerfile.build -t datamesh-builder .
docker create --name datamesh-builder datamesh-builder
docker cp datamesh-builder:/target/docker target/
docker cp datamesh-builder:/target/datamesh-server target/
docker rm -f datamesh-builder
docker build -t $(hostname).local:80/lukemarsden/datamesh-server:pushpull .

# allow disabling of registry push
if [ -z "${NO_PUSH}" ]; then
  docker push $(hostname).local:80/lukemarsden/datamesh-server:pushpull
fi
