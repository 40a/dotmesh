# dev commands

Commonly used commands for development locally - assumes Docker for Darwin or Linux.

## install dm cli command

```bash
$ sudo curl -o /usr/local/bin/dm \
    https://get.data-mesh.io/$(uname -s)/dm
$ sudo chmod +x /usr/local/bin/dm
```

## run api server without docker-compose

A local datamesh api server with frontend in dev trim.

#### build api server

This should be done initially and when you change anything in `cmd/datamesh-server`.

```bash
$ cd cmd/datamesh-server
$ export IMAGE=datamesh-server
$ NO_PUSH=1 bash rebuild.sh
```

 * `IMAGE` var to control what local image is built.
 * `NO_PUSH` to not push this

#### run

Start the server using the image you just built:

```bash
$ dm cluster init \
  --image ${IMAGE} \
  --allow-public-registration \
  --offline
```

## run api server with docker-compose

TBC

## reset

```bash
$ docker rm -f $(docker ps -aq)
$ rm -rf ~/.datamesh
$ docker volume rm datamesh-etcd-data
```
