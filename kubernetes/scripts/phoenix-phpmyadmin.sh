#!/bin/bash
kubectl port-forward -nmariadb svc/phpmyadmin-service 2023:2020
