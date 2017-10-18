#!/usr/bin/env bash
set -xe

IMAGE=${IMAGE:=$(hostname).local:80/datamesh/datamesh-server:latest}
PROVISIONER_IMAGE=${PROVISIONER_IMAGE:=$(hostname).local:80/datamesh/datamesh-dynamic-provisioner:latest}

mkdir -p target
docker build -f Dockerfile.build -t datamesh-builder .
docker create --name datamesh-builder datamesh-builder
docker cp datamesh-builder:/target/docker target/
docker cp datamesh-builder:/target/datamesh-server target/
docker cp datamesh-builder:/target/flexvolume target/
docker cp datamesh-builder:/target/dm-provisioner target/
docker rm -f datamesh-builder
docker build -t "${IMAGE}" .
docker build -f pkg/dynamic-provisioning/Dockerfile -t "${PROVISIONER_IMAGE}" .

# allow disabling of registry push
if [ -z "${NO_PUSH}" ]; then
	 docker push ${IMAGE}
	 docker push ${PROVISIONER_IMAGE}
fi
