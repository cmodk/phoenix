#!/bin/bash

NS=redis
KC="kubectl -n$NS"


for file in ./*.yaml
do
  if [[ -f $file ]]; then
    $KC apply -f $file
  fi
done


