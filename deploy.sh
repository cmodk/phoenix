#!/bin/bash

#./build_applications.sh  || exit
#./bin/phoenix-helper --cmd docker-build-images || exit

applications=`cd cmd && ls -d *`


for application in $applications; do

  echo "Building for $application"
  docker buildx build \
    --platform linux/amd64,linux/arm64 \
    -t eu.gcr.io/ae101-197818/sandbox/$application \
    --build-arg arg_application=$application \
    --progress=plain --no-cache \
    --target build .

done;


#./bin/linux/amd64/phoenix-helper --cmd recreate-pods --kubeconfig ~/.kube/config --namespace phoenix-sandbox --pod phoenix
