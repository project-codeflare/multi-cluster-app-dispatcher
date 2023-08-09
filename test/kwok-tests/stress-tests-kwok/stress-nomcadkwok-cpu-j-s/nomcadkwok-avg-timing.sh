#!/bin/bash

# clear the cluster for any potential jobs
echo "Deleting any jobs if any..." 
kubectl delete jobs --all -n default
echo " "

# clear any existing output directory containing yamls 
echo "Deleting any previous output directory if any..."
rm -r output_dir
echo " "

SCRIPT_DIR=$(readlink -f `dirname "${BASH_SOURCE[0]}"`)

function help() {
    echo "usage: stress-test [-h] [-j <jobs>] [-g <gpus>] [-a <awjobs>]"
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

# Function to extract the total time from the output
extract_total_time() {
    awk '/fake jobs without MCAD is: .* seconds/ {print $(NF-1)}' "$1"
}

# Track whether we have a valid kubectl login
echo "Checking whether we have a valid cluster login or not..."
check_kubectl_login_status

## Commented since running KWOK out of the cluster
# # Track whether you have the KWOK controller installed
# echo "Checking MCAD Controller installation status"
# echo
# check_kwok_installed_status


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

echo "jobs number is $jobs"
echo "Number of GPUs per pod: $gpus"
echo "Number of pods per job: $awjobs"

# Set the subdirectory name
subdirectory="output_dir"

# Create the output directory path
output_dir="$(pwd)/$subdirectory"

# Create the output directory if it doesn't exist
mkdir -p "$output_dir"

# This fixes the number of jobs to be one less so the for loop gets the right amount
((realjobs=$jobs-1))
for num in $(eval echo "{0.."$realjobs"}"); do
  next_num=$((num + 1))
  cp stress-nomcadkwok-cpu-j-s.yaml "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"

  # Had to do this OSTYPE because sed acts differently on Linux versus Mac
    case "$OSTYPE" in
      linux-gnu*)
        sed -i "s/nomcadkwok-cpu-job-short-0/nomcadkwok-cpu-job-short-$next_num/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i "s/parallelism: 1/parallelism: $awjobs/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i "s/completions: 1/completions: $awjobs/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml" ;;
      darwin*) 
        sed -i '' "s/nomcadkwok-cpu-job-short-0/nomcadkwok-cpu-job-short-$next_num/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i '' "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i '' "s/parallelism: 1/parallelism: $awjobs/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i '' "s/completions: 1/completions: $awjobs/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml" ;;
      *) 
        sed -i "s/nomcadkwok-cpu-job-short-0/nomcadkwok-cpu-job-short-$next_num/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i "s/nvidia.com\/gpu: 0/nvidia.com\/gpu: $gpus/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i "s/parallelism: 1/parallelism: $awjobs/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml"
        sed -i "s/completions: 1/completions: $awjobs/g" "$output_dir/stress-nomcadkwok-cpu-j-s-$next_num.yaml" ;;
    esac
done

# Define the number of iterations
iterations=1

# Perform the iterations
total_time=0
for ((i=1; i<=iterations; i++)); do
    # Run the script and capture the output in a temporary file
    output_file=$(mktemp)
    echo "Now calling nomcadkwok-stress-test.sh"
    echo " "
    ./nomcadkwok-stress-test.sh -j "$jobs" -g "$gpus" -a "$awjobs" > "$output_file"
    # Extract the total time from the output
    time=$(extract_total_time "$output_file")
    # Accumulate the total time
    total_time=$(bc <<< "$total_time + $time")
    # Remove the temporary file
    echo " "
    echo "Iteration $i complete"
    echo "Deleting all jobs for the fresh next run"
    ./cleanup-nomcadkwok-cpu-j-s.sh > "$output_file"
    rm "$output_file"
    echo " "
    sleep 10
done

# Calculate the average time
average_time=$(bc <<< "scale=2; $total_time / $iterations")
echo "Average time per iteration: $average_time seconds"

# Delete the output directory
rm -r "$output_dir"


