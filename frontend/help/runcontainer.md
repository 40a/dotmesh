## Running a container

To run a container with a datamesh volume:

```bash
$ docker run -d -v ${USER_NAME}/mydata:/var/lib/mysql \
    --volume-driver=dm --name=db \
    -e MYSQL_ROOT_PASSWORD=secret mysql
87fec9853dff4d6fad140e23e98845bda9faa7a1c8d63e
$ dm switch ${USER_NAME}/mydata
$ dm list
Current remote: default

  VOLUME                SERVER          BRANCH   CONTAINERS
* ${USER_NAME}/mydata   172.16.93.101   master   /db
$
```

Then write some data:


```bash
$ dm commit -m "No database"
$ docker run --link db:db -ti mysql \
    mysql -hdb -uroot -psecret
mysql> create database hello;
mysql> use hello;
mysql> create table countries (name varchar(255));
mysql> exit;
$ dm commit -m "Created hello database and countries table"
$ dm checkout -b newbranch
$ docker run --link db:db -ti mysql \
    mysql -hdb -uroot -psecret hello
mysql> insert into countries set name="england";
Query OK, 1 row affected (0.01 sec)

mysql> exit;
$ dm commit -m "Inserted england row"
$ dm checkout master
$ docker run --link db:db -ti mysql \
    mysql -hdb -uroot -psecret hello
mysql> select * from countries;
Empty set (0.00 sec)
```