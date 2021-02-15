#!/bin/bash

kubectl -nredis exec --stdin --tty redis-0 -- /bin/bash
