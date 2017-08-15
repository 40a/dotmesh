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

export CHROME_DRIVER_IMAGE=${CHROME_DRIVER_IMAGE:="blueimp/chromedriver"}
export CHROME_DRIVER_NAME=${CHROME_DRIVER_NAME:="datamesh-chromedriver"}

export NIGHTWATCH_IMAGE=${NIGHTWATCH_IMAGE:="datamesh-nightwatch"}
export NIGHTWATCH_NAME=${NIGHTWATCH_NAME:="datamesh-nightwatch"}

export DATAMESH_SERVER_NAME=${DATAMESH_SERVER_NAME:="datamesh-server-inner"}
export DATAMESH_SERVER_PORT=${DATAMESH_SERVER_PORT:="6969"}


function chromedriver-start() {
  docker run -d \
    --name ${CHROME_DRIVER_NAME} \
    -e VNC_ENABLED=true \
    -e EXPOSE_X11=true \
    ${CHROME_DRIVER_IMAGE}
}

function chromedriver-stop() {
  docker rm -f ${CHROME_DRIVER_NAME} || true
}

function frontend-test-build() {
  docker build -t ${NIGHTWATCH_IMAGE} -f ${DIR}/frontend/Dockerfile.nightwatch ${DIR}/frontend
}

function frontend-test() {
  docker run --rm \
    --name ${NIGHTWATCH_NAME} \
    --link ${CHROME_DRIVER_NAME}:chromedriver \
    --link ${DATAMESH_SERVER_NAME}:server \
    -e "LAUNCH_URL=server:${DATAMESH_SERVER_PORT}" \
    -e "WAIT_FOR_HOSTS=server:${DATAMESH_SERVER_PORT} chromedriver:4444 chromedriver:6060" \
    -v "${DIR}/frontend/.media/screenshots:/home/node/screenshots" \
    -v "${DIR}/frontend/.media/videos:/home/node/videos" \
    -v "${DIR}/frontend/test/specs:/home/node/specs" \
    -v "${DIR}/frontend/test/lib:/home/node/lib" \
    ${NIGHTWATCH_IMAGE} "${@}"
}

function cli-build() {
  echo "building datamesh CLI binary"
  cd "${DIR}/cmd/dm" && bash rebuild_docker.sh
}

function cluster-build() {
  echo "building datamesh server image: ${SERVER_IMAGE}"
  cd "${DIR}/cmd/datamesh-server" && IMAGE="${SERVER_IMAGE}" NO_PUSH=1 bash rebuild.sh
}

function cluster-prodbuild() {
  echo "building production datamesh server image: ${SERVER_IMAGE}"
  cp -r "${DIR}/frontend/dist" "${DIR}/cmd/datamesh-server/dist"
  cd "${DIR}/cmd/datamesh-server" && IMAGE="${SERVER_IMAGE}" NO_PUSH=1 bash mergebuild.sh
  rm -rf "${DIR}/cmd/datamesh-server/dist"
}

function cluster-start() {
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
  docker build -t ${FRONTEND_IMAGE} ${DIR}/frontend
}


function frontend-volumes() {
  local linkedVolumes=""
  declare -a frontend_volumes=("src" "www" "package.json" "webpack.config.js" "toolbox-variables.js" "yarn.lock")
  # always mount these for local development
  for volume in "${frontend_volumes[@]}"
  do
    linkedVolumes="${linkedVolumes} -v ${DIR}/frontend/${volume}:/app/${volume}"
  done
  # mount modules from templatestack for quick reloading
  # you need to have cloned https://github.com/binocarlos/templatestack.git to the same folder as datamesh for this to work
  if [ -n "${LINKMODULES}" ]; then
    linkedVolumes="${linkedVolumes} -v ${DIR}/../templatestack/template-tools:/app/node_modules/template-tools"
    linkedVolumes="${linkedVolumes} -v ${DIR}/../templatestack/template-ui:/app/node_modules/template-ui"
  fi
  echo "${linkedVolumes}"
}

function frontend-start() {
  local flags=""
  local linkedVolumes=$(frontend-volumes)
  if [ -n "${CLI}" ]; then
    flags=" --rm -ti --entrypoint bash"
  else
    flags=" -d"
  fi
  echo "running frontend dev server using ${FRONTEND_IMAGE}"
  docker run ${flags} \
    --name datamesh-frontend \
    --link datamesh-server-inner:datamesh-server \
    -p 8080:80 \
    -v "${DIR}/frontend:/app" \
    -v "/app/node_modules/" ${linkedVolumes} \
    ${FRONTEND_IMAGE}
}

function frontend-stop() {
  echo "stopping frontend dev server"
  docker rm -f datamesh-frontend
}

function frontend-dist() {
  docker run -it --rm \
    -v "${DIR}/frontend:/app" \
    ${FRONTEND_IMAGE} release
}

function build() {
  cli-build
  cluster-build
  frontend-build
}

function reset() {
  dm cluster reset
  docker rm -f datamesh-frontend || true
}

function usage() {
cat <<EOF
Usage:
  cli-build            rebuild the dm CLI
  cluster-build        rebuild the server image
  cluster-prodbuild    rebuild the server image with frontend code
  cluster-start        create a new cluster
  cluster-stop         stop a running cluster
  cluster-upgrade      update a cluster after build-server
  chromedriver-start   start chromedriver
  chromedriver-stop    stop chromedriver
  frontend-build       rebuild the frontend image
  frontend-start       start the frontend dev container
  frontend-stop        stop the frontend dev container
  frontend-dist        export the production build of the frontend
  frontend-test-build  build the frontend test image
  frontend-test        run the frontend tests
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
  cluster-prodbuild)   shift; cluster-prodbuild $@;;
  cluster-start)       shift; cluster-start $@;;
  cluster-stop)        shift; cluster-stop $@;;
  cluster-upgrade)     shift; cluster-upgrade $@;;
  chromedriver-start)  shift; chromedriver-start $@;;
  chromedriver-stop)   shift; chromedriver-stop $@;;
  frontend-test-build) shift; frontend-test-build $@;;
  frontend-test)       shift; frontend-test $@;;
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