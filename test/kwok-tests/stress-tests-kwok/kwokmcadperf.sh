#!/bin/bash

SCRIPT_DIR=$(readlink -f `dirname "${BASH_SOURCE[0]}"`)

function help() {
    echo "usage: kwokmcadperf.sh [-h]"
    echo
    echo "Description: Runs Appwrapper performance test script(s) in subdirectories under $SCRIPT_DIR."
    echo "NOTE: This runs on KWOK Fake nodes only."
    echo
    echo "Preconditions: "
    echo "   - The script assumes you've logged into your cluster already. If not, it will tell you to login."
    echo "   - The script checks that you have the mcad-controller installed, otherwise it'll tell you to install it first."
    echo "   - The script checks that you have the kwok-controller installed, otherwise it'll tell you to install it first."
    echo
    echo "Options:"
    echo "  -h       Print this help message"
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

function check_mcad_installed_status() {
    set +e
    kubectl get pod -A |grep mcad-controller &> /dev/null
    res2="$?"
    kubectl get crd |grep appwrapper &> /dev/null
    res3="$?"
    set -e
    MCAD="$res2"
    CRD="$res3"
      if [[ $MCAD == 1 ]] || [[ $CRD == 1 ]]
      then
        echo "You need Install MCAD Controller first before running this script"
        exit 1
      else
        echo "Nice, MCAD Controller is installed"
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


while getopts hf: option; do
    case $option in
        h)
            help
            exit 0
            ;;
        *)
            ;;
    esac
done
shift $((OPTIND-1))

# Track whether we have a valid kubectl login
echo "Checking whether we have a valid cluster login or not..."
check_kubectl_login_status

# Track whether you have the MCAD controller installed
echo "Checking MCAD Controller installation status"
echo
check_mcad_installed_status

# Track whether you have the KWOK controller installed
echo "Checking MCAD Controller installation status"
echo
check_kwok_installed_status

echo
read -p "How many fake KWOK appwrapper jobs do you want? " jobs
read -p "How many pods in a job? " awjobs
read -p "How many GPUs do you want to allocate per pod? " gpus

# Start the timer now
SECONDS=0

echo "jobs number is $jobs"
echo "Number of GPUs per pod: $gpus"
echo "Number of pods per AppWrapper: $awjobs"
export STARTTIME=`date +"%T"`
echo " "
echo "Jobs started at: $STARTTIME" |tee fake-job-$STARTTIME.log
echo " "

# This fixes the number of jobs to be one less so the for loop gets the right amount
((realjobs=$jobs-1))

for num in $(eval echo "{0.."$realjobs"}")
do
    next_num=$(($num + 1))
    echo "Submitting job $next_num"
# Had to do this OSTYPE because sed acts differently on Linux versus Mac
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/fake-defaultaw-schd-spec-with-timeout-$num/fake-defaultaw-schd-spec-with-timeout-$next_num/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/parallelism: 1/parallelism: $awjobs/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/completions: 1/completions: $awjobs/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
      darwin*) 
        sed -i '' "s/fake-defaultaw-schd-spec-with-timeout-$num/fake-defaultaw-schd-spec-with-timeout-$next_num/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i '' "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i '' "s/parallelism: 1/parallelism: $awjobs/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i '' "s/completions: 1/completions: $awjobs/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
      *) 
        sed -i "s/fake-defaultaw-schd-spec-with-timeout-$num/fake-defaultaw-schd-spec-with-timeout-$next_num/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/parallelism: 1/parallelism: $awjobs/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/completions: 1/completions: $awjobs/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
    esac
    kubectl apply -f ${SCRIPT_DIR}/preempt-exp-kwok.yaml
done

    # Let's reset the original preempt-exp-kwok.yaml file back to original value 
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/fake-defaultaw-schd-spec-with-timeout-$next_num/fake-defaultaw-schd-spec-with-timeout-1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml 
        sed -i "s/nvidia.com\/gpu: $gpus/nvidia.com\/gpu: 0/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/parallelism: $awjobs/parallelism: 1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/completions: $awjobs/completions: 1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
      darwin*) 
        sed -i '' "s/fake-defaultaw-schd-spec-with-timeout-$next_num/fake-defaultaw-schd-spec-with-timeout-1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i '' "s/nvidia.com\/gpu: $gpus/nvidia.com\/gpu: 0/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i '' "s/parallelism: $awjobs/parallelism: 1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i '' "s/completions: $awjobs/completions: 1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
      *) 
        sed -i "s/fake-defaultaw-schd-spec-with-timeout-$next_num/fake-defaultaw-schd-spec-with-timeout-1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/nvidia.com\/gpu: $gpus/nvidia.com\/gpu: 0/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/parallelism: $awjobs/parallelism: 1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml
        sed -i "s/completions: $awjobs/completions: 1/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
    esac

# Check for all jobs to report complete
jobstatus=`kubectl get jobs -n default --no-headers --field-selector status.successful=1 |wc -l`

while [ $jobstatus -lt $jobs ]
do
   echo "Number of completed jobs is: " $jobstatus " and the goal is: " $jobs
   sleep 1
   jobstatus=`kubectl get jobs -n default --no-headers --field-selector status.successful=1 |wc -l`
done

echo " "
export FINISHTIME=`date +"%T"`
echo "All $jobstatus jobs finished: $FINISHTIME" |tee -a fake-job-$STARTTIME.log
echo "Total amount of time for $jobs appwrappers is: $SECONDS seconds" |tee -a ${SCRIPT_DIR}/fake-job-$STARTTIME.log
echo " "
echo "Test results are stored in this file: ${SCRIPT_DIR}/fake-job-$next_num-$STARTTIME.log"

# Rename the log to show the number of jobs used
mv ${SCRIPT_DIR}/fake-job-$STARTTIME.log ${SCRIPT_DIR}/fake-job-$next_num-$STARTTIME.log

#Ask if you want to auto-cleanup the appwrapper jobs
echo "Do you want to cleanup the most recently created appwrappers? [Y/n]"
read DELETE
if [[ $DELETE == "Y" || $DELETE == "y" ]]; then
        echo "OK, deleting"
        ${SCRIPT_DIR}/cleanup-mcad-kwok.sh
else
        echo "OK, you'll need to cleanup yourself later using ./cleanup-mcad-kwok.sh"
fi
