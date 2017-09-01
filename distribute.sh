#!/usr/bin/env bash
set -e
if [ "$DATAMESH_DOCKER_IMAGE" == "" ]; then
    DATAMESH_DOCKER_IMAGE="$(hostname).local:80/lukemarsden/datamesh-server:pushpull"
    #DATAMESH_DOCKER_IMAGE="quay.io/lukemarsden/datamesh-server:pushpull"
fi
if [ "$1" == "" ]; then
    echo "Rebuilds and distributes datamesh on N nodes and localhost, assuming hostnames are in form <node-prefix><N>."
    echo "Usage: ./distribute.sh <node-prefix> <N> <arguments-passed-to-upgrade>"
    exit 1
fi
NODE_PREFIX=$1; shift
N=$1; shift
REMAINDER=$@
echo "====================="
echo "   BUILD DM CLIENT"
echo "====================="
make build-linux
echo "====================="
echo "   BUILD DM SERVER"
echo "====================="
docker build -t $DATAMESH_DOCKER_IMAGE ../datamesh-server
echo "====================="
echo "   COPY CLIENT LOCAL"
echo "====================="
sudo cp -v Linux/dm /usr/local/bin/dm
echo "====================="
echo "   PUSH SERVER IMAGE"
echo "====================="
docker push $DATAMESH_DOCKER_IMAGE
echo "====================="
echo "   COPY SERVER LOCAL"
echo "====================="
dm cluster upgrade --image $DATAMESH_DOCKER_IMAGE $@
echo "====================="
echo "   COPY CLIENT"
echo "====================="
for X in `seq 1 $N`; do
    (scp Linux/dm ${NODE_PREFIX}${X}:/tmp/dm &&
     ssh ${NODE_PREFIX}${X} 'sudo mv -v /tmp/dm /usr/local/bin/dm && \
        sudo chmod -v +x /usr/local/bin/dm') &
done
for P in `jobs -p`; do
    wait $P
done
echo "====================="
echo "   COPY SERVER"
echo "====================="
for X in `seq 1 $N`; do
    ssh ${NODE_PREFIX}${X} dm cluster upgrade --image $DATAMESH_DOCKER_IMAGE $@ &
done
for P in `jobs -p`; do
    wait $P
done
