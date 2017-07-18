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

function cli-build() {
  echo "building datamesh CLI binary"
  cd "${DIR}/cmd/dm" && bash rebuild_docker.sh
}

function cluster-build() {
  echo "building datamesh server image: ${SERVER_IMAGE}"
  cd "${DIR}/cmd/datamesh-server" && IMAGE="${SERVER_IMAGE}" NO_PUSH=1 bash rebuild.sh
}

function cluster-start() {
  echo "creating cluster using ${SERVER_IMAGE}"
  dm cluster init \
    --image ${SERVER_IMAGE} \
    --allow-public-registration \
    --frontend-proxy-container "datamesh-frontend" \
    --offline
}

function cluster-stop() {
  docker rm -f datamesh-server-inner
  docker rm -f datamesh-server
  docker rm -f datamesh-etcd
}

function cluster-upgrade() {
  echo "upgrading cluster using ${SERVER_IMAGE}"
  dm cluster upgrade \
    --image ${SERVER_IMAGE} \
    --allow-public-registration \
    --frontend-proxy-container "datamesh-frontend" \
    --offline
}

function frontend-build() {
  echo "building datamesh frontend image: ${FRONTEND_IMAGE}"
  cd "${DIR}/frontend" && IMAGE="${FRONTEND_IMAGE}" NO_PUSH=1 bash rebuild.sh
}

function frontend-start() {
  echo "running frontend dev server using ${FRONTEND_IMAGE}"
  docker run -d \
    --name datamesh-frontend \
    -v "${DIR}/frontend:/app" \
    -v "/app/node_modules/" \
    ${FRONTEND_IMAGE}
}

function frontend-stop() {
  echo "stopping frontend dev server"
  docker rm -f datamesh-frontend
}

function frontend-dist() {
  echo "build the production frontend code"
  docker rm -f datamesh-frontend
}

function build() {
  cli-build
  cluster-build
  frontend-build
}

function reset() {
  dm cluster reset
}

function usage() {
cat <<EOF
Usage:
  cli-build            rebuild the dm CLI
  cluster-build        rebuild the server image
  cluster-start        create a new cluster
  cluster-stop         stop a running cluster
  cluster-upgrade      update a cluster after build-server
  frontend-build       rebuild the frontend image
  frontend-start       start the frontend dev container
  frontend-stop        stop the frontend dev container
  frontend-dist        export the production build of the frontend
  build                rebuild all images
  reset                reset the cluster
  help                 display this message
EOF
  exit 1
}

function main() {
  case "$1" in
  cli-build)           shift; cli-build $@;;
  cluster-build)       shift; cluster-build $@;;
  cluster-start)       shift; cluster-start $@;;
  cluster-stop)        shift; cluster-stop $@;;
  cluster-upgrade)     shift; cluster-upgrade $@;;
  frontend-build)      shift; frontend-build $@;;
  frontend-start)      shift; frontend-start $@;;
  frontend-stop)       shift; frontend-stop $@;;
  frontend-dist)       shift; frontend-dist $@;;
  build)               shift; build $@;;
  reset)               shift; reset $@;;
  help)                shift; usage $@;;
  *)                   usage $@;;
  esac
}

main "$@"