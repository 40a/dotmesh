#!/usr/bin/env bash
set -xe

if [ `whoami` != 'root' ]; then
  echo >&2 "You must be root to do this."
  exit 1
fi

apt-get -y update
apt-get install -y docker.io zfsutils-linux jq curl golang

# make elastic search work
echo 'vm.max_map_count=262144' >> /etc/sysctl.conf
sysctl vm.max_map_count=262144

cat <<EOF > /etc/docker/daemon.json
{
    "storage-driver": "overlay2",
    "insecure-registries": ["$(hostname).local:80"]
}
EOF

cat <<EOF >> /etc/hosts
127.0.0.1 $(hostname).local
EOF

systemctl restart docker
adduser vagrant docker

curl -o /usr/local/bin/docker-compose -L "https://github.com/docker/compose/releases/download/1.15.0/docker-compose-$(uname -s)-$(uname -m)"
chmod +x /usr/local/bin/docker-compose
