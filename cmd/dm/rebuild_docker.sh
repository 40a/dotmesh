#!/bin/bash

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
export GOOS=${GOOS:="darwin"}

docker build -t datamesh-cli-builder -f Dockerfile.build .

OUTPUT_DIR="${DIR}/../../binaries/${GOOS}"

echo "building dm for ${GOOS} into ${OUTPUT_DIR}"

docker run -ti --rm \
  -v "${DIR}:/go/src/github.com/datamesh-io/datamesh/cmd/dm" \
  -v "${OUTPUT_DIR}:/target" \
  -e GOOS \
  -e CGO_ENABLED=0 \
  --entrypoint godep \
  datamesh-cli-builder \
  go build -a -installsuffix cgo -ldflags '-s' -o /target/dm .
