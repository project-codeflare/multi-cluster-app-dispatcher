#!/bin/bash

SCRIPT_DIR=$(readlink -f `dirname "${BASH_SOURCE[0]}"`)

# Set the subdirectory name
subdirectory="output_dir"

# Create the output directory path
output_dir="${SCRIPT_DIR}/${subdirectory}"

# Set default values
jobs=1
gpus=0
awjobs=1

# Parse command-line options
while getopts j:g:a: option; do
    case $option in
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


# This fixes the number of jobs to be one less so the for loop gets the right amount
((realjobs=$jobs-1))

# Start the timer now
SECONDS=0
export STARTTIME=`date +"%T"`

for num in $(eval echo "{0.."$realjobs"}") 
do
    next_num=$((num + 1))
    kubectl apply -f "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
done


# Check for all jobs to report complete
jobstatus=`kubectl get jobs -n default --no-headers --field-selector status.successful=$awjobs |grep nomcadkwok | wc -l`

while [ $jobstatus -lt $jobs ]
do
   echo "Number of completed jobs is: " $jobstatus " and the goal is: " $jobs
   sleep 1
   jobstatus=`kubectl get jobs -n default --no-headers --field-selector status.successful=$awjobs |grep nomcadkwok | wc -l`
done

# kubectl wait --for=condition=complete --timeout=-30s --all job

export FINISHTIME=`date +"%T"`
echo "Total amount of time for $jobs fake jobs without MCAD is: $SECONDS seconds" 
