#!/bin/bash
NS=phoenix-sandbox
kubectl -n$NS exec --stdin --tty deploy/phoenix-$1 -- /bin/sh
