#!/bin/bash
fly -t home set-pipeline -p phoenix -c ci/pipeline.yaml
