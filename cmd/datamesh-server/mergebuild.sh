#!/bin/bash
set -xe

IMAGE=${IMAGE:=$(hostname).local:80/lukemarsden/datamesh-server:pushpull}

docker build -t "${IMAGE}" -f Dockerfile.merge .

# allow disabling of registry push
if [ -z "${NO_PUSH}" ]; then
  docker push ${IMAGE}
fi
