#!/usr/bin/env bash
set -xe
cd cmd/dm
./rebuild.sh Linux
cd ../..
cd cmd/datamesh-server
./rebuild.sh
