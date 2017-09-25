#!/usr/bin/env bash
set -xe

cd $GOPATH/src/github.com/lukemarsden/datamesh/cmd/datamesh-server
./rebuild.sh
./prime-docker.sh
