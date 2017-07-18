# dev commands

How to develop the datamesh server, frontend and CLI all locally with Docker.

## building images

Before you begin, upgrade your Docker then build the required images:

```bash
$ bash dev.sh build
```

This will:

 * build the cluster container images
 * build the dm CLI and install it to `/usr/local/bin/dm`
 * build the frontend development image

You can run the three build stages seperately:

```bash
$ bash dev.sh cluster-build
$ bash dev.sh cli-build
$ bash dev.sh frontend-build
```

## run the stack

First - we bring up a datamesh cluster:

```bash
$ bash dev.sh cluster-start
```

This will start an etcd and 2 datamesh containers - `docker ps` will show this.

Then we bring up the frontend container:

```bash
$ bash dev.sh frontend-start
```

To attach to the frontend logs:

```bash
$ bash dev.sh frontend-logs
```

Now you should be able to:

```bash
$ open http://localhost:6969
```

and see the datamesh website.

## stop the stack

To stop a running stack - use these commands:

```bash
$ bash dev.sh cluster-stop
$ bash dev.sh frontend-stop
$ bash dev.sh reset
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
$ bash dev.sh cluster-build
```

Then we run the `upgrade` script which will replace in place the server container with our new image:

```bash
$ bash dev.sh cluster-upgrade
```

#### cli

Once you have edited the [cli code](cmd/dm) - run the build script:

```bash
$ bash dev.sh cli-build
```

This will build the go code in a container and output it to `binaries/$GOOS`.

We then move the binary to use it:

```bash
$ sudo mv binaries/darwin/dm /usr/local/bin/dm
$ sudo chmod +x /usr/local/bin/dm
```

#### frontend

The frontend is build using a [webpack config](frontend/webpack.config.js) and the local code is mounted as a volume which automatically triggers a rebuild when you save a file.

The code is mounted with a [webpack-hot-middleware](https://github.com/glenjamin/webpack-hot-middleware) server so if you are editing React components they should auto-reload in the browser.

If you are editing sagas, CSS or any of the non-visual part of the frontend, you will have to reload the browser.

#### rebuilding frontend image

There are times when you will need to rebuild the frontend image for example if you are adding an npm module.

First, stop the frontend server:

```bash
$ bash dev.sh frontend-stop
```

Then - use yarn to add the module:

```bash
$ cd frontend
$ yarn add my-new-kool-aid
$ cd ..
```

Build and start the frontend image:

```bash
$ bash dev.sh frontend-build
$ bash dev.sh frontend-start
```

#### building frontend production code

To build the production distribution of the frontend code:

```bash
$ bash dev.sh frontend-dist
```

This will create output in `frontend/dist` which can be copied into the server container for production.

TODO: add the `dist` folder to the production build of the datamesh server

