## Quickstart

### Install

```bash
sudo curl -o /usr/local/bin/dm \
    https://get.datamesh.io/$(uname -s)/dm
```

```bash
sudo chmod +x /usr/local/bin/dm
```

```bash
dm cluster init
```

### Connect Remote

```bash
dm remote add origin ${USER_NAME}@${SERVER_NAME}
```

Enter your password.

### Create a repository and push it

```bash
docker run -ti -v apples:/data busybox touch /data/my-data
```

```bash
dm switch apples
```

```bash
dm push origin apples
```
