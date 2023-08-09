#!/bin/bash

for i in `kubectl get appwrapper -n default |grep mcadkwok | awk '{print $1}'`; do kubectl delete appwrapper $i -n default ; done