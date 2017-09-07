#!/usr/bin/env bash
set -xe
./mark-cleanup.sh
./rebuild.sh
if [ -z "${SKIP_FRONTEND}" ]; then
    ./mergefrontend.sh
fi
