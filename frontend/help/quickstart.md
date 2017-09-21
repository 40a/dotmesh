## Quickstart

Get your local `dm` command connected to this datamesh cluster.

### Install Datamesh

These commands will install the `dm` binary onto your system:

```bash
sudo curl -o /usr/local/bin/dm \
    https://get.datamesh.io/$(uname -s)/dm
sudo chmod +x /usr/local/bin/dm
```

### Connect Remote

```bash
dm remote add mycluster ${USER_NAME}@${SERVER_NAME}
# enter password
dm remote switch mycluster
dm remote -v
```

### Create Volume

```bash
dm init apples
dm list
```
