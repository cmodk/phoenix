#!/bin/bash
NS=dev

if [ ! -z "$1" ]; then
  NS=$1
else
  echo "Need to specify namespace"
  exit
fi

SECRET=phoenix-certificates

kubectl delete secret $SECRET --namespace phoenix-$NS
kubectl create secret generic $SECRET \
  --from-file=./certificates/ca.pem \
  --from-file=./certificates/ca.key.pem \
  --from-file=./certificates/server.pem \
  --from-file=./certificates/server.key.pem \
  --namespace=phoenix-$NS
