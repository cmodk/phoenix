#!/bin/bash
NS=redis

kubectl -n$NS delete configmap redis-config
kubectl -n$NS create configmap redis-config --from-file=redis.conf

#Recreate the pods with new config
kubectl -n$NS delete pods -l app=redis


