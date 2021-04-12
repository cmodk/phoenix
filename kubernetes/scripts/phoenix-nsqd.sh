#!/bin/bash
kubectl port-forward -nnsq svc/nsqd 32150:32150
