#!/usr/bin/env bash

#
# the script that builds the frontend html, javascript and css (and images etc)
# 
# steps:
#
#   * build the image for the builder - datamesh-frontend-builder
#   * run the build script inside that image - NO volumes because gitlab root permissionsissue
#   * copy the build folder (dist) using docker cp
#   * gzip that folder and clean up
#

set -xe

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

mkdir -p target
docker build -f Dockerfile -t datamesh-frontend-builder .
docker rm -f datamesh-frontend-builder || true
docker run \
  --name datamesh-frontend-builder \
  datamesh-frontend-builder \
  release
docker cp datamesh-frontend-builder:/app/dist ./dist
docker rm -f datamesh-frontend-builder || true
ls -la ./dist
tar -cf dist.tar ./dist
gzip dist.tar
rm -rf ./dist
