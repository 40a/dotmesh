# dev commands

How to develop the datamesh server, frontend and CLI locally with Docker.

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

TODO: add the `dist` folder to the production build of the datamesh server


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

Whilst the local dev stack is running - you can run the frontend tests.

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

If you are running the production trim (where the frontend code is burnt into the server):

```bash
$ make chromedriver.start.prod
$ make frontend.test.prod
```


