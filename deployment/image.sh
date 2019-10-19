#!/bin/bash
set -x

project_root=$(cd ..; pwd)

cd ${project_root}

make images
