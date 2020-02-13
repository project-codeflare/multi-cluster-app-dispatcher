#!/bin/sh

set -x

apk update
apk upgrade

apk add make
apk add git
apk add bash
apk add libc-dev
apk add gcc

go env
cd .. && make run-test && make mcad-controller
