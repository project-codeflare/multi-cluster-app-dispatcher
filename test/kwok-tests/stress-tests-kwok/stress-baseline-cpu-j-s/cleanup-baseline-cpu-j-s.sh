#!/bin/bash

for i in `kubectl get job -n default |grep baseline-cpu-job-short | awk '{print $1}'`; do kubectl delete job $i -n default ; done
