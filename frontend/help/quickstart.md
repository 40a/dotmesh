## Quickstart

### Install

```bash
$ sudo curl -o /usr/local/bin/dm \
    https://get.datamesh.io/$(uname -s)/dm
$ sudo chmod +x /usr/local/bin/dm
$ dm cluster init
```

### Connect Remote

```bash
$ dm remote add mycluster ${USER_NAME}@${SERVER_NAME}
$ # enter password
$ dm remote switch mycluster
$ dm remote -v
```

### Create a Repository

```bash
$ dm init ${USER_NAME}/apples
$ dm list
```
