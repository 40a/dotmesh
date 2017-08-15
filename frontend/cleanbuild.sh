#!/bin/bash
set -xe

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

IMAGE=${IMAGE:=$(hostname).local:80/lukemarsden/datamesh-frontend-builder}

ls -la ${DIR}/dist

docker run --rm \
  -v ${DIR}/dist:/app/dist \
  --entrypoint sh \
  datamesh-frontend-builder \
  -c "ls -la /app/dist"

exit 1