#!/bin/bash
if [ "$1" == "" ]; then
    echo "Must specify Linux or Darwin as first argument"
    exit 1
fi
export PATH=/usr/local/go/bin:$PATH
set -xe
mkdir -p ../../binaries/$1
GOOS=${1,,} godep go build -i -o ../../binaries/$1/dm .
