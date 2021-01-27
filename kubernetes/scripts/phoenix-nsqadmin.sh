#!/bin/bash
kubectl port-forward -nnsq svc/nsqadmin 4171:4171
