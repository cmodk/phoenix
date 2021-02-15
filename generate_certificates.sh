#!/bin/bash
CN=""

if [ ! -z "$1" ]; then
  CN=$1
else
  echo "Need to specify common-name"
  exit
fi
docker run -it -v `pwd`/certificates:/usr/local/share/phoenix/certificates eu.gcr.io/ae101-197818/sandbox/phoenix-helper /usr/local/share/phoenix/phoenix-helper -certificate-generate -common-name $CN
