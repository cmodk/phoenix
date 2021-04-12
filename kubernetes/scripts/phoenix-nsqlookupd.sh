#!/bin/bash
kubectl port-forward -nnsq svc/nsqlookupd 4161:4161
