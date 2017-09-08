#!/usr/bin/env bash
for X in $(docker ps --format "{{.Names}}"|grep cluster-); do
    docker exec -ti $X touch /CLEAN_ME_UP
done
