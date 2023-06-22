#!/bin/bash

SCRIPT_DIR=$(readlink -f `dirname "${BASH_SOURCE[0]}"`)

function help() {
    echo "usage: nomcadkwokperf.sh [-h] [-j <jobs>] [-g <gpus>] [-a <awjobs>]"
    echo
    echo "Description: Runs performance test script(s) without MCAD in subdirectories under $SCRIPT_DIR."
    echo "NOTE: This runs on KWOK Fake nodes only."
    echo
    echo "Preconditions: "
    echo "   - The script assumes you've logged into your cluster already. If not, it will tell you to login."
    echo "   - The script checks that you have the kwok-controller installed, otherwise it'll tell you to install it first."
    echo
    echo "Options:"
    echo "  -h       Print this help message"
    echo "  -j <jobs>       Number of jobs to run (default: 1000)"
    echo "  -g <gpus>       Number of GPUs per job (default: 0)"
    echo "  -a <awjobs>     Number of pods per job (default: 1)"
    echo
}

function check_kubectl_login_status() {
    set +e
    kubectl get ns default &> /dev/null
    res="$?"
    set -e
    OCP="$res"
      if [ $OCP == 1 ]
      then
        echo "You need to login to your Kubernetes Cluster"
        exit 1
      else
        echo
        echo "Nice, looks like you're logged in"
      fi
}

function check_kwok_installed_status() {
    set +e
    kubectl get pod -A |grep kwok-controller &> /dev/null
    res2="$?"
    set -e
    KWOK="$res2"
      if [[ $KWOK == 1 ]]
      then
        echo "You need Install the KWOK Controller first before running this script"
        exit 1
      else
        echo "Nice, the KWOK Controller is installed"
    fi
}

# Set default values
jobs=10
gpus=0
awjobs=1

# Parse command-line options
while getopts hj:g:a: option; do
    case $option in
        h)
            help
            exit 0
            ;;
        j)
            jobs=$OPTARG
            ;;
        g)
            gpus=$OPTARG
            ;;
        a)
            awjobs=$OPTARG
            ;;
        *)
            ;;
    esac
done
shift $((OPTIND-1))

# Track whether we have a valid kubectl login
echo "Checking whether we have a valid cluster login or not..."
check_kubectl_login_status

# Track whether you have the KWOK controller installed
echo "Checking MCAD Controller installation status"
echo
check_kwok_installed_status

# Start the timer now
SECONDS=0

echo "jobs number is $jobs"
echo "Number of GPUs per pod: $gpus"
echo "Number of pods per job: $awjobs"
export STARTTIME=`date +"%T"`

# This fixes the number of jobs to be one less so the for loop gets the right amount
((realjobs=$jobs-1))

for num in $(eval echo "{0.."$realjobs"}")
do
    next_num=$(($num + 1))
    echo "Submitting job $next_num"
# Had to do this OSTYPE because sed acts differently on Linux versus Mac
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/noaw-kwok-job-$num/noaw-kwok-job-$next_num/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/parallelism: 1/parallelism: $awjobs/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/completions: 1/completions: $awjobs/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml ;;
      darwin*) 
        sed -i '' "s/noaw-kwok-job-$num/noaw-kwok-job-$next_num/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i '' "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i '' "s/parallelism: 1/parallelism: $awjobs/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i '' "s/completions: 1/completions: $awjobs/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml ;;
      *) 
        sed -i "s/noaw-kwok-job-$num/noaw-kwok-job-$next_num/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/parallelism: 1/parallelism: $awjobs/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/completions: 1/completions: $awjobs/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml ;;
    esac
    kubectl apply -f ${SCRIPT_DIR}/noaw-kwok-job.yaml
done

    # Let's reset the original noaw-kwok-job.yaml file back to original value 
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/noaw-kwok-job-$next_num/noaw-kwok-job-1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml 
        sed -i "s/nvidia.com\/gpu: $gpus/nvidia.com\/gpu: 0/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/parallelism: $awjobs/parallelism: 1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/completions: $awjobs/completions: 1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml ;;
      darwin*) 
        sed -i '' "s/noaw-kwok-job-$next_num/noaw-kwok-job-1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i '' "s/nvidia.com\/gpu: $gpus/nvidia.com\/gpu: 0/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i '' "s/parallelism: $awjobs/parallelism: 1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i '' "s/completions: $awjobs/completions: 1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml ;;
      *) 
        sed -i "s/noaw-kwok-job-$next_num/noaw-kwok-job-1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/nvidia.com\/gpu: $gpus/nvidia.com\/gpu: 0/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/parallelism: $awjobs/parallelism: 1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml
        sed -i "s/completions: $awjobs/completions: 1/g" ${SCRIPT_DIR}/noaw-kwok-job.yaml ;;
    esac

# Check for all jobs to report complete
jobstatus=`kubectl get jobs -n default --no-headers --field-selector status.successful=$awjobs |wc -l`

while [ $jobstatus -lt $jobs ]
do
   echo "Number of completed jobs is: " $jobstatus " and the goal is: " $jobs
   sleep 1
   jobstatus=`kubectl get jobs -n default --no-headers --field-selector status.successful=$awjobs |wc -l`
done

echo " "
export FINISHTIME=`date +"%T"`
echo "Total amount of time for $jobs fake jobs without MCAD is: $SECONDS seconds" 
