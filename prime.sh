#!/usr/bin/env bash
set -xe

cd $GOPATH/src/github.com/datamesh-io/datamesh/cmd/datamesh-server
./rebuild.sh

cd $GOPATH/src/github.com/datamesh-io/datamesh
./prime-docker.sh
