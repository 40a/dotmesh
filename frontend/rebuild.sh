#!/bin/bash
set -xe

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

IMAGE=${IMAGE:=$(hostname).local:80/lukemarsden/datamesh-frontend-builder}

mkdir -p target
docker build -f Dockerfile -t datamesh-frontend-builder .
#docker run --rm \
#  datamesh-frontend-builder \
#  release

echo "TODO - run the release without volumes (use cp)"

