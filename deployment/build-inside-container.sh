#!/bin/sh

set -x

apk add make
apk add git

cd .. && make generate-code && make kar-controller
