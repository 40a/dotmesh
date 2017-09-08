#!/bin/bash
set -xe

if [ `whoami` != 'vagrant' ]; then
  echo >&2 "You must be the vagrant user to do this."
  exit 1
fi

mkdir -p $HOME/.ssh
cat <<EOF > $HOME/.ssh/config
Host github.com
  StrictHostKeyChecking no
  UserKnownHostsFile=/dev/null
Host neo.lukemarsden.net
  StrictHostKeyChecking no
  UserKnownHostsFile=/dev/null
EOF

if [ -z "${GOPATH}" ]; then
  export GOPATH=/home/vagrant/gocode
  export PATH=$PATH:/usr/lib/go-1.8/bin
  echo "export GOPATH=${GOPATH}" >> $HOME/.bash_profile
  echo "export PATH=\$PATH:/usr/lib/go-1.8/bin:$GOPATH/bin" >> $HOME/.bash_profile
fi

mkdir -p $GOPATH

if [ ! -d "$GOPATH/src/github.com/lukemarsden/datamesh" ]; then
  mkdir -p $GOPATH/src/github.com/lukemarsden
  cd $GOPATH/src/github.com/lukemarsden
  git clone git@neo.lukemarsden.net:root/datamesh
fi

if [ ! -d "$HOME/kubernetes" ]; then
  cd $HOME/
  git clone git@github.com:kubernetes/kubernetes
  cd kubernetes
  git clone git@github.com:lukemarsden/kubeadm-dind-cluster dind
fi

if [ ! -d "$HOME/datamesh-instrumentation" ]; then
  cd $HOME/
  git clone git@github.com:lukemarsden/datamesh-instrumentation
  cd datamesh-instrumentation
fi

cd $HOME/datamesh-instrumentation
./up.sh secret # where secret is some local password

if [ ! -d "$HOME/discovery.datamesh.io" ]; then
  cd $HOME/
  git clone git@github.com:lukemarsden/discovery.datamesh.io
fi

cd $HOME/discovery.datamesh.io
./start-local.sh

cd $GOPATH/src/github.com/lukemarsden/datamesh
./prime.sh

go get github.com/tools/godep