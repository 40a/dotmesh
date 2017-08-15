#!/bin/bash
set -xe

export IMAGE=${IMAGE:=$(hostname).local:80/lukemarsden/datamesh-frontend-test-runner:pushpull}
export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

docker build -t ${IMAGE} -f ${DIR}/test/Dockerfile ${DIR}/test

# allow disabling of registry push
if [ -z "${NO_PUSH}" ]; then
  docker push ${IMAGE}
fi
