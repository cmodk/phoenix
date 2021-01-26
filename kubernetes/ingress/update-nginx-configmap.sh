#!/bin/bash
if [ -z "$1" ]; then
  echo "Missing namespace argument"
  exit
fi

NS=$1

kubectl -nphoenix-$NS delete configmap nginx-conf
kubectl -nphoenix-$NS create configmap nginx-conf --from-file=nginx.conf=nginx-loadbalancer-config-$NS.conf

#Recreate the pods with new config
kubectl -nphoenix-$NS delete pods -l app=loadbalancer


