#!/usr/bin/env bash

if [ "$ADMIN_PW" == "" ]; then
    echo "Please set ADMIN_PW"
    exit 1
fi

if [ "$IFACE" == "" ]; then
    IFACE="eth0"
    echo "Using iface $IFACE, specify IFACE to override"
fi

nodes=`docker ps --filter 'name=^/cluster_.*$' --format "{{.Names}}"`

# bash 4 only, associative array for passwords
declare -A passwords

# Get passwords from containers

for node in $nodes; do
    passwords[$node]=`docker exec -ti $node cat /root/.datamesh/config | jq .Remotes.local.ApiKey`
    passwords[$node]=`echo ${passwords[$node]} |tr -d '"'`
done

declare -A ips

for node in $nodes; do
    ips[$node]=`docker exec -ti $node ifconfig $IFACE | grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1`
done

echo "******************************************************************************"
echo "If running on a headless VM, try:"
echo "    docker run -d --name=tinyproxy --net=host dannydirect/tinyproxy:latest ANY"
echo "Then configure your web browser to proxy all HTTP traffic through your VM's IP"
echo "on port 8888."
echo "******************************************************************************"

for node in $nodes; do
    docker exec -i $node  \
        docker run --restart=always -d -v /root:/root \
            --name etcd-browser -p 0.0.0.0:8000:8000 \
            --env ETCD_HOST=${ips[$node]} -e ETCD_PORT=42379 \
            -e ETCDCTL_CA_FILE=/root/.datamesh/pki/ca.pem \
            -e ETCDCTL_KEY_FILE=/root/.datamesh/pki/apiserver-key.pem \
            -e ETCDCTL_CERT_FILE=/root/.datamesh/pki/apiserver.pem \
            -t -i $(hostname).local:80/lukemarsden/etcd-browser:v1 > /dev/null 2>/dev/null &
done

echo "Kibana:                                           http://admin:$ADMIN_PW@localhost:83/"

# node uis
for node in $nodes; do
    echo "$node cluster:     http://admin:${passwords[$node]}@${ips[$node]}:6969/ux"
    echo "$node frontend:    http://admin:${passwords[$node]}@${ips[$node]}:8080/ui"
done

for job in `jobs -p`; do
    wait $job
done

# etcd viewers

for node in $nodes; do
    echo $node etcd viewer: http://${ips[$node]}:8000/
done

# debug
for node in $nodes; do
    docker exec -ti $node \
        docker exec -ti datamesh-server-inner \
            curl http://localhost:6060/debug/pprof/goroutine?debug=1 > $X.goroutines
done
