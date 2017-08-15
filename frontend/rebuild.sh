#!/bin/bash
set -xe

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

IMAGE=${IMAGE:=$(hostname).local:80/lukemarsden/datamesh-frontend-builder}

mkdir -p target
docker build -f Dockerfile -t datamesh-frontend-builder .
docker run --rm \
  -v ${DIR}/dist:/app/dist \
  datamesh-frontend-builder \
  release

# allow disabling of registry push
if [ -z "${NO_PUSH}" ]; then
  docker push ${IMAGE}
fi
