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
    echo "   - The script checks that you have the mcad-controller installed (or the newer CodeFlare Operator), otherwise it'll tell you to install it first."
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
    kubectl get pod -A |grep codeflare-operator-manager &> /dev/null
    res4="$?"
    set -e
    MCAD="$res2"
    CRD="$res3"
    CODEFLARE="$res4"
      if [[ $MCAD == 1 ]] && [[ $CODEFLARE == 1 ]] || [[ $CRD == 1 ]]
      then
        echo "You need Install the MCAD Controller or the latest CodeFlare Operator first before running this script"
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
read -p "How many fake KWOK appwrapper jobs do you want?" JOBS

# Start the timer now
SECONDS=0

echo "jobs number is $jobs"
export STARTTIME=`date +"%T"`
echo " "
echo "Jobs started at: $STARTTIME" |tee fake-job-$STARTTIME.log
echo " "

COUNTER=1
while [ $COUNTER -le $JOBS ]
do
    ORIG_COUNTER=$(($COUNTER - 1))
    echo "Submitting job $COUNTER"

# Had to do this OSTYPE because sed acts differently on Linux versus Mac
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/fake-defaultaw-schd-spec-with-timeout-$ORIG_COUNTER/fake-defaultaw-schd-spec-with-timeout-$COUNTER/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
      darwin*)
        sed -i '' "s/fake-defaultaw-schd-spec-with-timeout-$ORIG_COUNTER/fake-defaultaw-schd-spec-with-timeout-$COUNTER/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
      *)
        sed -i "s/fake-defaultaw-schd-spec-with-timeout-$ORIG_COUNTER/fake-defaultaw-schd-spec-with-timeout-$COUNTER/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
    esac
    kubectl apply -f ${SCRIPT_DIR}/preempt-exp-kwok.yaml
COUNTER=$[$COUNTER +1]
done


    # Let's reset the original preempt-exp-kwok.yaml file back to original value 
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/fake-defaultaw-schd-spec-with-timeout-$JOBS/fake-defaultaw-schd-spec-with-timeout-0/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
      darwin*) 
        sed -i '' "s/fake-defaultaw-schd-spec-with-timeout-$JOBS/fake-defaultaw-schd-spec-with-timeout-0/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
      *) 
        sed -i "s/fake-defaultaw-schd-spec-with-timeout-$JOBS/fake-defaultaw-schd-spec-with-timeout-0/g" ${SCRIPT_DIR}/preempt-exp-kwok.yaml ;;
    esac

# Check for all jobs to report complete
JOBSTATUS=`kubectl get jobs -n default --no-headers --field-selector status.successful=1 |wc -l`

while [ $JOBSTATUS -lt $JOBS ]
do
   echo "Number of completed jobs is: " $JOBSTATUS " and the goal is: " $JOBS
   sleep 10
   JOBSTATUS=`kubectl get jobs -n default --no-headers --field-selector status.successful=1 |wc -l`
done

echo " "
export FINISHTIME=`date +"%T"`
echo "All $JOBSTATUS jobs finished: $FINISHTIME" |tee -a fake-job-$STARTTIME.log
echo "Total amount of time for $JOBS appwrappers is: $SECONDS seconds" |tee -a ${SCRIPT_DIR}/fake-job-$STARTTIME.log
echo " "
echo "Test results are stored in this file: ${SCRIPT_DIR}/fake-job-$JOBS-$STARTTIME.log"

# Rename the log to show the number of jobs used
mv ${SCRIPT_DIR}/fake-job-$STARTTIME.log ${SCRIPT_DIR}/fake-job-$JOBS-$STARTTIME.log

#Ask if you want to auto-cleanup the appwrapper jobs
echo "Do you want to cleanup the most recently created appwrappers? [Y/n]"
read DELETE
if [[ $DELETE == "Y" || $DELETE == "y" ]]; then
        echo "OK, deleting"
        ${SCRIPT_DIR}/cleanup-mcad-kwok.sh
else
        echo "OK, you'll need to cleanup yourself later using ./cleanup-mcad-kwok.sh"
fi
