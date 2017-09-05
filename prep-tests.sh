#!/usr/bin/env bash
./mark-cleanup.sh
./rebuild.sh
if [ -z "${SKIP_FRONTEND}" ]; then
    ./mergefrontend.sh
fi
