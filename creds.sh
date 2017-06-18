#!/bin/bash

if [ "$ADMIN_PW" == "" ]; then
    echo "Please set ADMIN_PW"
    exit 1
fi

if [ "$IFACE" == "" ]; then
    IFACE="eth0"
    echo "Using iface $IFACE, specify IFACE to override"
fi

node1=`docker ps --filter 'name=^/node_.*_1$' --format "{{.Names}}" |head -n 1`
node2=`docker ps --filter 'name=^/node_.*_2$' --format "{{.Names}}" |head -n 1`

# Get passwords from containers

node1pw=`docker exec -ti $node1 cat /root/.datamesh/config | jq .Remotes.local.ApiKey`
node2pw=`docker exec -ti $node2 cat /root/.datamesh/config | jq .Remotes.local.ApiKey`

node1pw=`echo $node1pw |tr -d '"'`
node2pw=`echo $node2pw |tr -d '"'`

node1ip=`docker exec -ti $node1 ifconfig $IFACE | grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1`
node2ip=`docker exec -ti $node2 ifconfig $IFACE | grep "inet addr" | cut -d ':' -f 2 | cut -d ' ' -f 1`

echo ===
echo node1pw: $node1pw
echo node2pw: $node2pw
echo ===

for X in node1 node2; do
    ipvar=${X}ip
    docker exec -i ${!X} \
        docker run --restart=always -d -v /root:/root \
            --name etcd-browser -p 0.0.0.0:8000:8000 \
            --env ETCD_HOST=${!ipvar} -e ETCD_PORT=42379 \
            -e ETCDCTL_CA_FILE=/root/.datamesh/pki/ca.pem \
            -e ETCDCTL_KEY_FILE=/root/.datamesh/pki/apiserver-key.pem \
            -e ETCDCTL_CERT_FILE=/root/.datamesh/pki/apiserver.pem \
            -t -i $(hostname).local:80/lukemarsden/etcd-browser:v1 &
done

# kibana
xdg-open http://admin:$ADMIN_PW@localhost:83/
sleep 0.4

# node uis
xdg-open http://admin:$node1pw@$node1ip:6969/ux
sleep 0.4
xdg-open http://admin:$node2pw@$node2ip:6969/ux
sleep 0.4

for job in `jobs -p`; do
    wait $job
done

# etcd viewers

xdg-open http://$node1ip:8000/
sleep 0.4
xdg-open http://$node2ip:8000/

# debug
for X in $node1 $node2; do
    docker exec -ti $X \
        docker exec -ti datamesh-server-inner \
            curl http://localhost:6060/debug/pprof/goroutine?debug=1 > $X.goroutines
done
