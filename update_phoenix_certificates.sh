#!/bin/bash
NS=dev

if [ ! -z "$1" ]; then
  NS=$1
else
  echo "Need to specify namespace"
  exit
fi

SECRET=phoenix-certificates
KUBECTL="kubectl -n$NAMESPACE"

$KUBECTL delete secret $SECRET
$KUBECTL create secret generic $SECRET \
  --from-file=./certificates/ca.pem \
  --from-file=./certificates/ca.key.pem \
  --from-file=./certificates/server.pem \
  --from-file=./certificates/server.key.pem

#Refresh pods to load new certificate
$KUBECTL -nphoenix-production get pods -o name |grep phoenix  |xargs $KUBECTL delete  
