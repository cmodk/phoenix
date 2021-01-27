#!/bin/bash
kubectl port-forward -nmariadb svc/mariadb 3306:3306
