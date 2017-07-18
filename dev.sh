#!/bin/bash -e
#
# scripts to manage the developers local installation of datamesh
#

set -e

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

export DATABASE_ID=${DATABASE_ID:=""}
export SERVER_IMAGE=${SERVER_IMAGE:="datamesh-server:latest"}
export FRONTEND_IMAGE=${FRONTEND_IMAGE:="datamesh-frontend:latest"}
export DATAMESH_HOME=${DATAMESH_HOME:="~/.datamesh"}

function cluster-build() {
  echo "building datamesh server image: ${SERVER_IMAGE}"
  cd "${DIR}/cmd/datamesh-server" && IMAGE="${SERVER_IMAGE}" NO_PUSH=1 bash rebuild.sh
}

function cluster-start() {
  echo "removing existing PKI keys"
  rm -rf ~/.datamesh
  echo "creating cluster using ${SERVER_IMAGE}"
  dm cluster init \
    --image ${SERVER_IMAGE} \
    --allow-public-registration \
    --offline
}

function cluster-stop() {
  docker rm -f datamesh-server-inner
  docker rm -f datamesh-server
  docker rm -f datamesh-etcd
  rm -rf "${DATAMESH_HOME}/pki"
}

function cluster-upgrade() {
  echo "upgrading cluster using ${SERVER_IMAGE}"
  dm cluster upgrade \
    --image ${SERVER_IMAGE} \
    --allow-public-registration \
    --offline
}

function frontend-build() {
  echo "building datamesh frontend image: ${FRONTEND_IMAGE}"
  cd "${DIR}/frontend" && IMAGE="${FRONTEND_IMAGE}" NO_PUSH=1 bash rebuild.sh
}

function frontend-start() {
  echo "running frontend dev server using ${FRONTEND_IMAGE}"
  docker run -ti --rm \
    --net host \
    --name datamesh-frontend \
    -v "${DIR}/frontend:/app/frontend" \
    -v "/app/frontend/node_modules/" \
    ${FRONTEND_IMAGE}
}

function frontend-stop() {
  echo "stopping frontend dev server"
  docker rm -f datamesh-frontend
}

function frontend-exec() {
  local cmd="$@"
  if [ -z "${cmd}" ]; then
    cmd='bash'
  fi
  docker exec -ti datamesh-frontend "${cmd}"
}

function build() {
  echo "building all images"
  cluster-build
  frontend-build
}

function reset() {
  dm cluster reset
}

function usage() {
cat <<EOF
Usage:
  cluster-build        rebuild the server image
  cluster-start        create a new cluster
  cluster-stop         stop a running cluster
  cluster-upgrade      update a cluster after build-server
  frontend-build       rebuild the frontend image
  frontend-start       start the frontend dev container
  frontend-stop        stop the frontend dev container
  frontend-exec        run a command in the frontend container
  build                rebuild all images
  reset                reset the cluster
  help                 display this message
EOF
  exit 1
}

function main() {
  case "$1" in
  cluster-build)       shift; cluster-build $@;;
  cluster-start)       shift; cluster-start $@;;
  cluster-stop)        shift; cluster-stop $@;;
  cluster-upgrade)     shift; cluster-upgrade $@;;
  frontend-build)      shift; frontend-build $@;;
  frontend-start)      shift; frontend-start $@;;
  frontend-stop)       shift; frontend-stop $@;;
  frontend-exec)       shift; frontend-exec $@;;
  build)               shift; build $@;;
  reset)               shift; reset $@;;
  help)                shift; usage $@;;
  *)                   usage $@;;
  esac
}

main "$@"