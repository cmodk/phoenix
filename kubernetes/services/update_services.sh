#!/bin/bash

if [ -z "$1" ]; then
  echo "Missing namespace argument"
  exit
fi

NS=$1
KC="kubectl -nphoenix-$NS"


for file in ./*.yaml
do
  if [[ -f $file ]]; then
    $KC apply -f $file
  fi
done


