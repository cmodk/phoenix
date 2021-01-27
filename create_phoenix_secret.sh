#!/bin/bash
NS=dev

if [ ! -z "$1" ]; then
  NS=$1
else
  echo "Need to specify namespace"
  exit
fi


kubectl delete secret phoenix-config --namespace phoenix-$NS
kubectl create secret generic phoenix-config --from-file=./config/$NS.yaml --namespace=phoenix-$NS
