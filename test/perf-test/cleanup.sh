#!/bin/bash

for i in `kubectl get appwrappers.workload.codeflare.dev -n default |grep defaultaw | awk '{print $1}'`; do kubectl delete appwrappers.workload.codeflare.dev $i -n default ; done
