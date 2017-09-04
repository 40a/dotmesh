# developing datamesh via integration tests

## intro

This is the recommended way to develop Datamesh backend code, where you care
about exercising multi-node or multi-cluster behaviour (e.g. federated
push/pull).

We will use the Datamesh acceptance test suite, which starts a set of
docker-in-docker environments, one for each node in your cluster and each
cluster in your federation (as configured by the integration test(s) you choose
to run).

The test suite intentionally leaves the last docker-in-docker environments
running so that you can do ad-hoc poking or log/trace viewing after running a
test (using `debug-in-browser.sh`).

This acceptance test suite uses docker-in-docker, kubeadm style. It creates
docker containers which simulate entire computers, each running systemd, and
then uses 'dm cluster init', etc, to set up datamesh. After the initial setup
and priming of docker images, which takes quite some time, it should take ~60
seconds to spin up a 2 node datamesh cluster to run a test. It then does not
require internet access.

## setup - nixos

Use a [config like this](https://github.com/lukemarsden/nixos/blob/master/vmware-guest/configuration.nix) for `/etc/nixos/configuration.nix`.

Then run:
```
sudo nixos-rebuild switch
```

## setup - ubuntu

[Install Docker](https://docs.docker.com/engine/installation/linux/docker-ce/ubuntu/), then put the following docker config in /etc/docker/daemon.json:

```
{
    "storage-driver": "overlay2",
    "insecure-registries": ["$(hostname).local:80"]
}
```

Replacing `$(hostname)` with your hostname, and then `systemctl restart docker`.

Run (as root):
```
apt install zfsutils-linux jq
echo 'vm.max_map_count=262144' >> /etc/sysctl.conf
sysctl vm.max_map_count=262144
```

[Install Docker Compose](https://docs.docker.com/compose/install/).

## setup

Assuming you have set your GOPATH (e.g. to `$HOME/gocode`):

```
mkdir -p $GOPATH/src/github.com/lukemarsden
cd $GOPATH/src/github.com/lukemarsden
git clone git@neo.lukemarsden.net:root/datamesh
```

We're going to create `~/kubernetes`, `~/datamesh-instrumentation` and
`~/discovery.datamesh.io` directories:

```
cd ~/
git clone git@github.com:kubernetes/kubernetes
cd kubernetes
git clone git@github.com:lukemarsden/kubeadm-dind-cluster dind
cd ~/
git clone git@github.com:lukemarsden/datamesh-instrumentation
cd datamesh-instrumentation
./up.sh secret # where secret is some local password
```

The `datamesh-instrumentation` pack includes ELK for logging, Zipkin for
tracing, a local registry which is required for the integration tests, and an
etcd-browser which is useful for inspecting the state in your test clusters'
etcd instances.

```
cd ~/
git clone git@github.com:lukemarsden/discovery.datamesh.io
cd discovery.datamesh.io
./start-local.sh
```

The `discovery.datamesh.io` server provides a discovery service for datamesh
clusters, we need to run a local one to make the tests work offline.

You have to do some one-off setup and priming of docker images before these
tests will run:

```
cd $GOPATH/src/github.com/lukemarsden/datamesh
./prime.sh
```

Now install some deps:
```
go get github.com/tools/godep
```

## running tests

To run the test suite, run:

```
cd $GOPATH/src/github.com/lukemarsden/datamesh
./mark-cleanup.sh; ./rebuild.sh && ./test.sh
```

To run just an individual set of tests, run:
```
./mark-cleanup.sh; ./rebuild.sh && ./test.sh -run TestTwoSingleNodeClusters
```

To run an individual test, specify `TestTwoSingleNodeClusters/TestName` for
example.

To open a bunch of debug tools (Kibana for logs, Zipkin for traces, and etcd
browsers for each cluster's etcd), run (where 'secret' is the pasword you
specified when you ran `up.sh` in `datamesh-instrumentation`):

```
ADMIN_PW=secret ./debug-in-browser.sh
```

# single server local dev

How to develop a single datamesh server, frontend and CLI locally with Docker.

*These instructions are most useful for developing the Javascript frontend.*

## building images

Before you begin, upgrade your Docker then build the required images:

```bash
$ make build
```

This will:

 * build the cluster container images
 * build the dm CLI and install it to `/usr/local/bin/dm`
 * build the frontend development image

You can run the three build stages seperately:

```bash
$ make cluster.build
$ make cli.build
$ make frontend.build
```

## run the stack

#### start cluster
First we bring up a datamesh cluster:

```bash
$ make cluster.start
```

This will start an etcd and 2 datamesh containers - `docker ps` will show this.

#### start frontend
Then we bring up the frontend container (which proxies back to the cluster for api requests):

```bash
$ make frontend.start
```

To attach to the frontend logs:

```bash
$ make frontend.logs
```

Now you should be able to open the app in your browser:

```bash
$ open http://localhost:8080
```

To view the new UI:

```bash
$ open http://localhost:8080/ui
```

If you want to see the cluster server directly - you can:

```bash
$ open http://localhost:6969
```

#### frontend CLI

Sometimes it's useful to have the frontend container hooked up but with a bash prompt:

```bash
$ make frontend.dev
$ yarn run watch
```

#### linking templatestack

The `template-ui` and `template-tools` npm modules are used in the UI and to iterate quickly it can be useful to have these linked up to the hot reloading.

To do this - you first need to clone https://github.com/binocarlos/templatestack.git to the same folder as datamesh then:

```bash
$ make frontend.link
$ yarn run watch
```

Now - any changes made to `templatestack/template-ui` will hot-reload.

#### reset & boot errors

If anything happens which results in the cluster not being able to boot - usually the solution is:

```bash
$ make reset
```

which does:

```bash
$ dm cluster reset
```

## stop the stack

To stop a running stack - use these commands:

```bash
$ make cluster.stop
$ make frontend.stop
$ make reset
```

We currently need to reset the cluster each time we stop - this means you will have to re-create your user account when you restart the stack.

Normally this is not too painful because you can rebuild the server code and `upgrade` the cluster (read below).

TODO: allow existing admin password / possibly PKI key data to be used when bringing up the cluster
TODO: have user defined fixture files than can quickly create user accounts for local dev

## changing code

Here is how you would edit code for the 3 main sections of datamesh:

 * [server](cmd/datamesh-server) - container name = `datamesh-server`
 * [cli](cmd/dm) - binary location = `/usr/local/bin/dm`
 * [frontend](frontend) - container name = `datamesh-frontend`

#### server

Once you have edited the [server code](cmd/datamesh-server) - run the build script:

```bash
$ make cluster.build
```

Then we run the `upgrade` script which will replace in place the server container with our new image:

```bash
$ make cluster.upgrade
```

#### cli

Once you have edited the [cli code](cmd/dm) - run the build script:

```bash
$ make cli.build
```

This will build the go code in a container and output it to `binaries/$GOOS`.

We then move the binary to use it:

```bash
$ sudo mv -f binaries/darwin/dm /usr/local/bin/dm
$ sudo chmod +x /usr/local/bin/dm
```

#### frontend

The frontend is built using a [webpack config](frontend/webpack.config.js) and the local code is mounted as a volume which automatically triggers a rebuild when you save a file.

The code is mounted with a [webpack-hot-middleware](https://github.com/glenjamin/webpack-hot-middleware) server so if you are editing React components they should auto-reload in the browser.

If you are editing sagas, CSS or any of the non-visual part of the frontend, you will have to reload the browser.

#### rebuilding frontend image

There are times when you will need to rebuild the frontend image for example if you are adding an npm module.

First, stop the frontend server:

```bash
$ make frontend.stop
```

Then - use yarn to add the module:

```bash
$ cd frontend
$ yarn add my-new-kool-aid
$ cd ..
```

Build and start the frontend image:

```bash
$ make frontend.build
$ make frontend.start
```

You can `docker exec -ti datamesh-frontend bash` to get a CLI inside the frontend container to run any other commands.

#### building frontend production code

To build the production distribution of the frontend code:

```bash
$ make frontend.dist
```

This will create output in `frontend/dist` which can be copied into the server container for production.

The frontend `dist` folder is merged into the datamesh-server image in the `merge` CI job.

## running production mode

To run the frontend code in production mode (i.e. static files inside the server) - do the following:

```bash
$ make prod
```

This will:

```bash
$ make frontend.build
$ make frontend.dist
$ make cluster.build
$ make cluster.prodbuild
$ make cluster.start
```

and end up with the same as `cluster.start` but with the frontend code built into the server.

The difference in this mode is you need to hit `localhost:6969` to see it in the browser.

## running frontend tests

TODO: the frontend tests require interaction between `dm`

It is useful to run the frontend tests against a running hot-reloading development env.

First - startup chromedriver and build the test image.

```bash
$ make frontend.test.build # only needed once
$ make chromedriver.start
```

Then - as the frontend is rebuilding as you make changes - you can re-run the test suite:

```bash
$ make frontend.test
```

Videos & screenshots are produced after each test run - they live in `frontend/.media`

#### production trim

If you are running the production trim (where the frontend code is burnt into the server):

```bash
$ make chromedriver.start.prod
$ make frontend.test.prod
```
