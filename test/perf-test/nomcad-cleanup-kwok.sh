#!/bin/bash

for i in `kubectl get job -n default |grep noaw-kwok-job | awk '{print $1}'`; do kubectl delete job $i -n default ; done
