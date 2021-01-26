#!/bin/bash
KC="kubectl -nmariadb"

$KC delete configmap mariadb-config
$KC create configmap mariadb-config --from-file=galera.cnf=./galera.cnf --from-file=start-mariadb-instance.sh=start-mariadb-instance.sh --from-file=my.cnf=./my.cnf --from-file=bootstrap_cluster.sh=bootstrap_cluster.sh
