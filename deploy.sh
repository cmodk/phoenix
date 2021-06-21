#!/bin/bash
./build_applications.sh  || exit

./bin/phoenix-helper --cmd docker-build-images || exit

./bin/phoenix-helper --cmd recreate-pods --kubeconfig ~/.kube/config --namespace phoenix-sandbox --pod phoenix
