#!/bin/bash

read -p "How many appwrapper jobs do you want?" jobs

# Start the timer now
SECONDS=0

echo "jobs number is $jobs"
export STARTTIME=`date +"%T"`
echo " "
echo "Jobs started at: $STARTTIME" |tee job-$STARTTIME.log
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
        sed -i "s/defaultaw-schd-spec-with-timeout-$num/defaultaw-schd-spec-with-timeout-$next_num/g" preempt-exp.yaml ;;
      darwin*) 
        sed -i '' "s/defaultaw-schd-spec-with-timeout-$num/defaultaw-schd-spec-with-timeout-$next_num/g" preempt-exp.yaml ;;
      *) 
        sed -i "s/defaultaw-schd-spec-with-timeout-$num/defaultaw-schd-spec-with-timeout-$next_num/g" preempt-exp.yaml ;;
    esac
    oc apply -f preempt-exp.yaml
done

    # Let's reset the original preempt-exp.yaml file back to original value 
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/defaultaw-schd-spec-with-timeout-$next_num/defaultaw-schd-spec-with-timeout-1/g" preempt-exp.yaml ;;
      darwin*) 
        sed -i '' "s/defaultaw-schd-spec-with-timeout-$next_num/defaultaw-schd-spec-with-timeout-1/g" preempt-exp.yaml ;;
      *) 
        sed -i "s/defaultaw-schd-spec-with-timeout-$next_num/defaultaw-schd-spec-with-timeout-1/g" preempt-exp.yaml ;;
    esac

# Check for all jobs to report complete
jobstatus=`oc get jobs --no-headers --field-selector status.successful=1 |wc -l`

while [ $jobstatus -lt $jobs ]
do
   echo "Number of completed jobs is: " $jobstatus " and the goal is: " $jobs
   sleep 10
   jobstatus=`oc get jobs --no-headers --field-selector status.successful=1 |wc -l`
done

echo " "
export FINISHTIME=`date +"%T"`
echo "All $jobstatus jobs finished: $FINISHTIME" |tee -a job-$STARTTIME.log
echo "Total amount of time for $jobs appwrappers is: $SECONDS seconds" |tee -a job-$STARTTIME.log
echo " "
echo "Test results are stored in this file: job-$next_num-$STARTTIME.log"

# Rename the log to show the number of jobs used
mv job-$STARTTIME.log job-$next_num-$STARTTIME.log

#Ask if you want to auto-cleanup the appwrapper jobs
echo "Do you want to cleanup the most recently created appwrappers? [Y/n]"
read DELETE
if [[ $DELETE == "Y" || $DELETE == "y" ]]; then
        echo "OK, deleting"
        ./cleanup.sh 
else
        echo "OK, you'll need to cleanup yourself later using ./cleanup.sh"
fi
