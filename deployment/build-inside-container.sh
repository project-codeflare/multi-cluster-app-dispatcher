#!/bin/sh

set -x

apk add make
apk add git

cd .. && make mcad-controller
