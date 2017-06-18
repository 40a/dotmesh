#!/bin/bash
for X in $(docker ps --format "{{.Names}}"|grep node_); do
    docker exec -ti $X touch /CLEAN_ME_UP
done
