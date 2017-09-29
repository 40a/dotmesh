#!/bin/bash
set -xe

export IMAGE=${IMAGE:=$(hostname).local:80/datamesh/datamesh-frontend-test-runner:latest}
export CHROMEDRIVER_IMAGE=${CHROMEDRIVER_IMAGE:=$(hostname).local:80/datamesh/datamesh-chromedriver:latest}
export GOTTY_IMAGE=${GOTTY_IMAGE:=$(hostname).local:80/datamesh/datamesh-gotty:latest}
export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

docker build -t ${IMAGE} -f ${DIR}/test/Dockerfile ${DIR}/test
docker build -t ${CHROMEDRIVER_IMAGE} -f ${DIR}/test/Dockerfile.chromedriver ${DIR}/test
docker build -t ${GOTTY_IMAGE} -f ${DIR}/test/Dockerfile.gotty ${DIR}/test

# allow disabling of registry push
if [ -z "${NO_PUSH}" ]; then
  docker push ${IMAGE}
  docker push ${CHROMEDRIVER_IMAGE}
  docker push ${GOTTY_IMAGE}
fi
