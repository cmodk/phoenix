#!/bin/bash
kubectl -nscylladb logs -l app=scylladb -f
