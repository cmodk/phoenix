#!/bin/bash
kubectl scale -n phoenix-sandbox deployment phoenix-$1 --replicas=$2
