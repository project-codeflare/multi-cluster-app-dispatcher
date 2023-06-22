#!/bin/bash

# Define the number of iterations
iterations=10

# Function to extract the total time from the output
extract_total_time() {
    awk '/fake jobs without MCAD is: .* seconds/ {print $(NF-1)}' "$1"
}

# Set default values
jobs=50
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

# Perform the iterations
total_time=0
for ((i=1; i<=iterations; i++)); do
    # Run the script and capture the output in a temporary file
    output_file=$(mktemp)
    ./nomcadkwokperf.sh -j "$jobs" -g "$gpus" -a "$awjobs" > "$output_file"
    # Extract the total time from the output
    time=$(extract_total_time "$output_file")
    # Accumulate the total time
    total_time=$(bc <<< "$total_time + $time")
    # Remove the temporary file
    echo " "
    echo "Iteration $i complete"
    echo "Deleting all jobs for the fresh next run"
    ./nomcad-cleanup-kwok.sh > "$output_file"
    rm "$output_file"
    echo " "
    sleep 60
done

# Calculate the average time
average_time=$(bc <<< "scale=2; $total_time / $iterations")
echo "Average time per iteration: $average_time seconds"
