#!/bin/bash
[[ $(hostname) =~ -([0-9]+)$ ]] || exit 1
server_id=${BASH_REMATCH[1]}
datadir=/var/lib/mysql/$(hostname)
bootstrap=`cat ${datadir}/grastate.dat | grep safe_to_bootstrap | awk '{print $2}'`

### We exited, if this is maria-0 or maria-1, try to bootstrap a new cluster
#if [ ${server_id} -lt 2 ]; then
if [ $bootstrap -eq 1 ]; then
  echo "Initializing new cluster"
  /usr/local/bin/docker-entrypoint.sh mysqld --wsrep-new-cluster --datadir=$datadir
else
  ## Try just to join the cluster
  /usr/local/bin/docker-entrypoint.sh mysqld --datadir=$datadir
  if [ $? -ne 0 ]; then
    echo "Maria cluster has failed, start with recover option"
    /usr/local/bin/docker-entrypoint.sh mysqld --datadir=$datadir --wsrep-recover
  fi
fi
