#!/bin/bash
kubectl -nphoenix-sandbox logs -l app=phoenix-events -f
