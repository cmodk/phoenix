#!/bin/bash
kubectl -nphoenix-sandbox logs -l app=phoenix-devices -f
