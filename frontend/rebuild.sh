#!/bin/bash
set -xe

IMAGE=${IMAGE:=$(hostname).local:80/lukemarsden/datamesh-frontend:pushpull}

docker build -f Dockerfile -t ${IMAGE} .

# allow disabling of registry push
if [ -z "${NO_PUSH}" ]; then
  docker push ${IMAGE}
fi
