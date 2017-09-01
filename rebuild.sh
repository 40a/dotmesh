#!/usr/bin/env bash
cd cmd/dm
export PATH=/usr/local/go/bin:$PATH
set -xe
mkdir -p ../../binaries/Linux && GOOS=linux godep go build -o ../../binaries/Linux/dm .
cd ../..
cd cmd/datamesh-server
./rebuild.sh
