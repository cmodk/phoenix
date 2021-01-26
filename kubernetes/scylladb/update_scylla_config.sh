#!/bin/bash
KC="kubectl -nscylladb"

$KC delete configmap scylladb-config
$KC create configmap scylladb-config --from-file=scylla.yaml=./scylla.yaml --from-file=cassandra-rackdc.properties=cassandra-rackdc.properties
