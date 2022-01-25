#!/bin/bash

#./build_applications.sh  || exit
#./bin/phoenix-helper --cmd docker-build-images || exit



docker buildx build \
  --platform linux/amd64 \
  -t cmodk/phoenix:sandbox \
  . || exit



#./bin/linux/amd64/phoenix-helper --cmd recreate-pods --kubeconfig ~/.kube/config --namespace phoenix-sandbox --pod phoenix
