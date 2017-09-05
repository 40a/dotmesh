#!/usr/bin/env bash
# embeds the frontend into the local datamesh server
# run me after ./rebuild.sh if you want a working frontend build
set -xe
(cd frontend && ./rebuild.sh)
rm -rf cmd/datamesh-server/dist && cp -a frontend/dist cmd/datamesh-server/
(cd cmd/datamesh-server && ./mergebuild.sh)
