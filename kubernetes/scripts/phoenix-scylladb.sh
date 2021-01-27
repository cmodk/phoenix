#!/bin/bash
kubectl port-forward -nscylladb svc/scylladb 9042:9042
