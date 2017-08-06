#!/bin/bash
set -xe

function fetch_zfs {
    KERN=$(uname -r)
    RELEASE=zfs-${KERN}.tar.gz
    cd /bundled-lib
    if [ -d /bundled-lib/lib/modules ]; then
        # Try loading a cached module (which we cached in a docker
        # volume)
        depmod -b /bundled-lib || true
        if modprobe -d /bundled-lib zfs; then
            echo "Successfully loaded cached ZFS for $KERN :)"
            return
        else
            echo "Unable to load cached module, trying to fetch one (maybe you upgraded your kernel)..."
            mv /bundled-lib/lib /bundled-lib/lib.backup-`date +%s`
        fi
    fi
    if ! curl -f -o ${RELEASE} https://get.datamesh.io/zfs/${RELEASE}; then
        echo "ZFS is not installed on your docker host, and unable to find a kernel module for your kernel: $KERN"
        echo "Please create a new GitHub issue, pasting this error message, and tell me which Linux distribution you are using, at:"
        echo
        echo "    https://github.com/lukemarsden/dm/issues"
        echo
        echo "Meanwhile, you should still be able to use datamesh if you install ZFS manually on your host system by following the instructions at http://zfsonlinux.org/ and then re-run the datamesh installer."
        echo
        echo "Alternatively, Ubuntu 16.04 and later comes with ZFS preinstalled, so using that should Just Work. Kernel modules for Docker for Mac and other Docker distributions are also provided."
        exit 1
    fi
    tar xf ${RELEASE}
    depmod -b /bundled-lib || true
    modprobe -d /bundled-lib zfs
    echo "Successfully loaded downloaded ZFS for $KERN :)"
}

# Put the data file inside /var/lib/docker so that we end up on the big
# partition if we're in a boot2docker env
DIR=${USE_POOL_DIR:-/var/lib/docker/datamesh}
FILE=${DIR}/datamesh_data
POOL=${USE_POOL_NAME:-pool}
MOUNTPOINT=${MOUNTPOINT:-$DIR/mnt}

echo "=== Using mountpoint $MOUNTPOINT"

# Docker volume where we can cache downloaded, "bundled" zfs
BUNDLED_LIB=/bundled-lib
# Bind-mounted system library where we can attempt to modprobe any
# system-provided zfs modules (e.g. Ubuntu 16.04) or those manually installed
# by user
SYSTEM_LIB=/system-lib

# Set up mounts that are needed
nsenter -t 1 -m -u -n -i sh -c \
    'set -xe
    if [ $(mount |grep '$MOUNTPOINT' |wc -l) -eq 0 ]; then
        echo "Creating and bind-mounting shared '$MOUNTPOINT'"
        mkdir -p '$MOUNTPOINT' && \
        mount --bind '$MOUNTPOINT' '$MOUNTPOINT' && \
        mount --make-shared '$MOUNTPOINT';
    fi
    mkdir -p /run/docker/plugins
    mkdir -p /var/datamesh'

if [ ! -e /sys ]; then
    mount -t sysfs sys sys/
fi

if [ ! -d $DIR ]; then
    mkdir -p $DIR
fi
if ! modinfo zfs >/dev/null 2>&1; then
    depmod -b /system-lib || true
    if ! modprobe -d /system-lib zfs; then
        fetch_zfs
    else
        echo "Successfully loaded system ZFS :)"
    fi
else
    echo "ZFS already loaded :)"
fi
if [ ! -e /dev/zfs ]; then
    mknod -m 660 /dev/zfs c $(cat /sys/class/misc/zfs/dev |sed 's/:/ /g')
fi
if ! zpool status $POOL; then
    if [ ! -f $FILE ]; then
        truncate -s 10G $FILE
        zpool create -m $MOUNTPOINT $POOL $FILE
    else
        zpool import -f -d $DIR $POOL
    fi
fi

# Clear away stale socket if existing
rm -f /run/docker/plugins/dm.sock

# At this point, if we try and run any 'docker' commands and there are any
# datamesh containers already on the host, we'll deadlock because docker will
# go looking for the dm plugin. So, we need to start up a fake dm plugin which
# just responds immediately with errors to everything. It will create a socket
# file which will hopefully get clobbered by the real thing.
datamesh-server --temporary-error-plugin &

# Attempt to avoid the race between `temporary-error-plugin` and the real
# datamesh-server. If `--temporary-error-plugin` loses the race, the
# plugin is broken forever.
while [ ! -e /run/docker/plugins/dm.sock ]; do
    echo "Waiting for /run/docker/plugins/dm.sock to exist due to temporary-error-plugin..."
    sleep 0.1
done

# Clear away old running server if running
docker rm -f datamesh-server-inner || true

echo "Starting the 'real' datamesh-server in a sub-container. Go check 'docker logs datamesh-server-inner' if you're looking for datamesh logs."

log_opts=""
rm_opt=""
if [ "$LOG_ADDR" != "" ]; then
    log_opts="--log-driver=syslog --log-opt syslog-address=tcp://$LOG_ADDR:5000"
    rm_opt="--rm"
fi

# To have its port exposed on Docker for Mac, `docker run` needs -p 30969.  But
# datamesh-server also wants to discover its routeable IPv4 addresses (on Linux
# anyway; multi-node clusters work only on Linux because we can't discover the
# Mac's IP from a container).  So to work with both we do that in the host
# network namespace (via docker) and pass it in.
YOUR_IPV4_ADDRS="$(docker run -i --net=host $DATAMESH_DOCKER_IMAGE datamesh-server --guess-ipv4-addresses)"

pki_volume_mount=""
if [ "$PKI_PATH" != "" ]; then
    pki_volume_mount="-v $PKI_PATH:/pki"
fi

net="-p 30969:30969"
link=""
if [ "$DATAMESH_ETCD_ENDPOINT" == "" ]; then
    # If etcd endpoint is overridden, then don't try to link to a local
    # datamesh-etcd container (etcd probably is being provided externally, e.g.
    # by etcd operator on Kubernetes).
    link="--link datamesh-etcd:datamesh-etcd"
fi
if [ "$DATAMESH_ETCD_ENDPOINT" != "" ]; then
    # When running in a pod network, calculate the id of the current container
    # in scope, and pass that as --net=container:<id> so that datamesh-server
    # itself runs in the same network namespace.
    self_containers=$(docker ps -q --filter="ancestor=$DATAMESH_DOCKER_IMAGE")
    array_containers=( $self_containers )
    num_containers=${#array_containers[@]}
    if [ $num_containers -eq 0 ]; then
        echo "Cannot find id of own container!"
        exit 1
    fi
    if [ $num_containers -gt 1 ]; then
        echo "Found more than one id of own container! $self_containers"
        exit 1
    fi
    net="--net=container:$self_containers"
fi

secret=""
if [[ "$INITIAL_ADMIN_PASSWORD_FILE" != "" && -e $INITIAL_ADMIN_PASSWORD_FILE ]]; then
    # shell escape the password, https://stackoverflow.com/questions/15783701
    pw=$(cat $INITIAL_ADMIN_PASSWORD_FILE |sed -e "s/'/'\\\\''/g")
    secret="-e 'INITIAL_ADMIN_PASSWORD=$pw'"
fi

docker run -i $rm_opt --privileged --name=datamesh-server-inner \
    -v /var/lib/docker:/var/lib/docker \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /run/docker/plugins:/run/docker/plugins \
    -v $MOUNTPOINT:$MOUNTPOINT:rshared \
    -v /var/datamesh:/var/datamesh \
    -l traefik.port=30969 \
    -l traefik.frontend.rule=Host:cloud.datamesh.io \
    $net \
    $link \
    -e "PATH=$PATH" \
    -e "LD_LIBRARY_PATH=$LD_LIBRARY_PATH" \
    -e "MOUNT_PREFIX=$MOUNTPOINT" \
    -e "POOL=$POOL" \
    -e "YOUR_IPV4_ADDRS=$YOUR_IPV4_ADDRS" \
    -e "ALLOW_PUBLIC_REGISTRATION=$ALLOW_PUBLIC_REGISTRATION" \
    -e "ASSETS_URL_PREFIX=$ASSETS_URL_PREFIX" \
    -e "HOMEPAGE_URL=$HOMEPAGE_URL" \
    -e "TRACE_ADDR=$TRACE_ADDR" \
    -e "DATAMESH_ETCD_ENDPOINT=$DATAMESH_ETCD_ENDPOINT" \
    $secret \
    $log_opts \
    $pki_volume_mount \
    -v datamesh-kernel-modules:/bundled-lib \
    $DATAMESH_DOCKER_IMAGE \
    "$@" >/dev/null
