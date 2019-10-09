#!/bin/bash

cp ../_output/bin/kar-controllers .
docker build --tag k8s-spark-mcm-dispatcher-master-1:8443/xqueuejob-controller:v1.13 -f Dockerfile.both .
rm ./kar-controllers

docker push k8s-spark-mcm-dispatcher-master-1:8443/xqueuejob-controller:v1.13
