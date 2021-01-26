#!/bin/bash
[[ $(hostname) =~ -([0-9]+)$ ]] || exit 1
server_id=${BASH_REMATCH[1]}
datadir=/var/lib/mysql/$(hostname)
bootstrap=`cat ${datadir}/grastate.dat | grep safe_to_bootstrap | awk '{print $2}'`

echo "safe_to_bootstrap: $bootstrap"

## Only allow maria-0 to be the very first node in the cluster
if [ ${server_id} -eq 0 ]; then
  sed -i 's/safe_to_bootstrap.*/safe_to_bootstrap: 1/' $datadir/grastate.dat
  echo "Initializing new cluster"
  /usr/local/bin/docker-entrypoint.sh mysqld --wsrep-new-cluster --datadir=$datadir
else
  exit -1
fi
