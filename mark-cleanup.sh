#!/usr/bin/env bash
set -xe
for X in $(docker ps --format "{{.Names}}" | grep cluster- || true); do
    docker exec -ti $X touch /CLEAN_ME_UP
done
