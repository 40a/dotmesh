#!/bin/bash
set -xe

IMAGE=${IMAGE:=$(hostname).local:80/datamesh/datamesh-server:latest}

docker build -t "${IMAGE}" -f Dockerfile.merge .

# allow disabling of registry push
if [ -z "${NO_PUSH}" ]; then
  docker push ${IMAGE}
fi
