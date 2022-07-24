#!/bin/bash
kubectl -nphoenix-sandbox scale statefulsets phoenix-mqtt --replicas=$1
