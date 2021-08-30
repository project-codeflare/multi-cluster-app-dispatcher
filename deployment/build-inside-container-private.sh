#!/bin/sh

# Verify paramenter
if [ "$#" -ne "2" ]
then
    echo "Illegal number of parameters, requires 2."
    echo " "
    exit 2
fi
GIT_UID=${1:-GIT_UID}
GIT_TOKEN=${2:-GIT_TOKEN}

set -x

apk update
apk upgrade

apk add make
apk add git
apk add bash
apk add libc-dev
apk add gcc

go env

set +x
# Set the github uid and token for https access. Used for go.mod/private.mod
git config --global url."https://${GIT_UID}:${GIT_TOKEN}@github.ibm.com/".insteadOf "https://github.ibm.com/"
set -x

cd ..  && BUILD_TAG=private BUILD_GOPRIVATE=github.ibm.com/* make mcad-controller && make run-test
