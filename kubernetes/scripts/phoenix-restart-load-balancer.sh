#!/bin/bash
NAMESPACE=phoenix-sandbox
KUBECTL="kubectl -n$NAMESPACE"
$KUBECTL -n${NAMESPACE} get pods -o name |grep loadbalancer  |xargs $KUBECTL delete
