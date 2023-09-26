#!/bin/bash

SCRIPT_DIR=$(readlink -f `dirname "${BASH_SOURCE[0]}"`)

function help() {
    echo "usage: perf.sh [-h]"
    echo
    echo "Description: Runs Appwrapper performance test script(s) in subdirectories under $SCRIPT_DIR."
    echo
    echo "Preconditions: "
    echo "   - The script assumes you've logged into your cluster already. If not, it will tell you to login."
    echo "   - The script checks that you have the mcad-controller installed (or the newer CodeFlare Operator), otherwise it'll tell you to install it first."
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
#check_mcad_installed_status

echo
read -p "How many appwrapper jobs do you want?" JOBS

# Start the timer now
SECONDS=0

echo "Appwrapper number is $jobs"
export STARTTIME=`date +"%T"`
echo " "
echo "Appwrappers started at: $STARTTIME" |tee job-$STARTTIME.log
echo " "

COUNTER=1
while [ $COUNTER -le $JOBS ]
do
    ORIG_COUNTER=$(($COUNTER - 1))
    echo "Submitting job $COUNTER"

# Had to do this OSTYPE because sed acts differently on Linux versus Mac
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/defaultaw-schd-spec-with-timeout-$ORIG_COUNTER/defaultaw-schd-spec-with-timeout-$COUNTER/g" ${SCRIPT_DIR}/preempt-exp.yaml ;;
      darwin*)
        sed -i '' "s/defaultaw-schd-spec-with-timeout-$ORIG_COUNTER/defaultaw-schd-spec-with-timeout-$COUNTER/g" ${SCRIPT_DIR}/preempt-exp.yaml ;;
      *)
        sed -i "s/defaultaw-schd-spec-with-timeout-$ORIG_COUNTER/defaultaw-schd-spec-with-timeout-$COUNTER/g" ${SCRIPT_DIR}/preempt-exp.yaml ;;
    esac
    kubectl apply -f ${SCRIPT_DIR}/preempt-exp.yaml
COUNTER=$[$COUNTER +1]
done

    # Let's reset the original preempt-exp.yaml file back to original value 
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/defaultaw-schd-spec-with-timeout-$JOBS/defaultaw-schd-spec-with-timeout-0/g" ${SCRIPT_DIR}/preempt-exp.yaml ;;
      darwin*) 
        sed -i '' "s/defaultaw-schd-spec-with-timeout-$JOBS/defaultaw-schd-spec-with-timeout-0/g" ${SCRIPT_DIR}/preempt-exp.yaml ;;
      *) 
        sed -i "s/defaultaw-schd-spec-with-timeout-$JOBS/defaultaw-schd-spec-with-timeout-0/g" ${SCRIPT_DIR}/preempt-exp.yaml ;;
    esac

# Check for all appwrappers to report complete
jobstatus=`kubectl get appwrappers -o=custom-columns=SUCCESS:.status.Succeeded -n default |grep 1 |wc -l`

while [ $JOBSTATUS -lt $JOBS ]
do
   echo "Number of completed appwrappers is: " $jobstatus " and the goal is: " $jobs
   sleep 10
   jobstatus=`kubectl get appwrappers -o=custom-columns=SUCCESS:.status.Succeeded -n default |grep 1 |wc -l`
done

echo " "
export FINISHTIME=`date +"%T"`
echo "All $jobstatus appwrappers finished: $FINISHTIME" |tee -a job-$STARTTIME.log
echo "Total amount of time for $jobs appwrappers is: $SECONDS seconds" |tee -a ${SCRIPT_DIR}/job-$STARTTIME.log
echo " "
echo "Test results are stored in this file: ${SCRIPT_DIR}/job-$JOBS-$STARTTIME.log"

# Rename the log to show the number of jobs used
mv ${SCRIPT_DIR}/job-$STARTTIME.log ${SCRIPT_DIR}/job-$JOBS-$STARTTIME.log

#Ask if you want to auto-cleanup the appwrapper jobs
echo "Do you want to cleanup the most recently created appwrappers? [Y/n]"
read DELETE
if [[ $DELETE == "Y" || $DELETE == "y" ]]; then
        echo "OK, deleting"
        ${SCRIPT_DIR}/cleanup.sh 
else
        echo "OK, you'll need to cleanup yourself later using ./cleanup.sh"
fi
