import random
import time
import subprocess
from datetime import datetime
import os
import argparse
import json

# Function to check job status
def check_job_status(num_pod_per_job):
    command = ["kubectl", "get", "jobs", "-n", "default", "--no-headers", "--field-selector", f"status.successful={num_pod_per_job}"]
    result = subprocess.run(command, capture_output=True, text=True)
    output = result.stdout.strip()
    completed_jobs = len(output.splitlines())
    return completed_jobs

def generate_output(output_file):
    # Define column names
    column_names = ["NAME", "CREATION_TIME", "COMPLETION_TIME", "GPU_NEEDED", "SLEEP_TIME"]

    # Run kubectl command and capture output
    output_command = subprocess.run(["kubectl", "get", "jobs", "-o", "json"], capture_output=True, text=True)
    output = output_command.stdout.strip()

    # Parse JSON output
    data = json.loads(output)

    # Extract and format desired columns
    job_results = []
    job_results.append("\t".join(column_names))  # Add column names as the first row
    for item in data.get("items", []):
        name = item["metadata"]["name"]
        creation_timestamp = item["metadata"]["creationTimestamp"]
        completion_time = item["status"].get("completionTime", "")
        gpu_needed = item["spec"]["template"]["spec"]["containers"][0]["resources"]["limits"].get("nvidia.com/gpu", "")
        sleep_time = item["spec"]["template"]["spec"]["containers"][0]["args"][1]

        # Append formatted row to results list
        job_results.append(f"{name}\t{creation_timestamp}\t{completion_time}\t{gpu_needed}\t{sleep_time}")

    # Join results with newlines
    formatted_output = "\n".join(job_results)

    # Write output to file
    with open(output_file, "w") as file:
        file.write(formatted_output)


# Function to generate job requests
def generate_job_request():
    if random.random() < 2/3:  # 2/3 probability of requiring 1 GPU
        gpu_needed = 1
    else:
        gpu_needed = 8

    return gpu_needed

# Function to generate YAML file for job request
def generate_yaml(job_id, gpu_required, sleep_time):
    yaml_template = f"""
apiVersion: batch/v1
kind: Job
metadata:
  namespace: default
  name: job-{job_id}
spec:
  parallelism: 1
  completions: 1
  template:
    metadata:
      namespace: default
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: type
                    operator: In
                    values:
                      - kwok
      tolerations:
        - key: "kwok.x-k8s.io/node"
          operator: "Exists"
          effect: "NoSchedule"
      containers:
      - args:
          - sleep
          - {sleep_time}
        name: job-{job_id}
        image: nginx:1.24.0
        resources:
            limits:
              cpu: 5m
              memory: 20M
              nvidia.com/gpu: {gpu_required}
            requests:
              cpu: 5m
              memory: 20M
              nvidia.com/gpu: {gpu_required}
      restartPolicy: Never
"""
    return yaml_template

# Parse command line arguments
parser = argparse.ArgumentParser(description="Generate job requests in a Kubernetes system.")
parser.add_argument("--mean-arrival", type=int, default=50, help="Mean arrival time for job requests in seconds")
parser.add_argument("--total-jobs", type=int, default=100, help="Total number of job requests to generate")
parser.add_argument("--job-size", type=int, default=60, help="Mean sleep time of container in seconds")
parser.add_argument("--output-file", type=str, default="job_results.txt", help="Output file to store job results")
args = parser.parse_args()

# Set random seed
random.seed(datetime.now().timestamp())

# Parameters from command line arguments
mean_arrival = args.mean_arrival
total_jobs = args.total_jobs
job_size = args.job_size
output_file = args.output_file

# Simulate job requests
for i in range(total_jobs):
    # Simulate job request arrival based on Poisson process
    inter_arrival_time = random.expovariate(1 / mean_arrival)
    print("Next arrival after " + str(inter_arrival_time) + " seconds \n")
    time.sleep(inter_arrival_time)

    # Generate job request
    gpu_required = generate_job_request()

    # Generate job size taken from exponential distribution
    sleep_time = str(random.expovariate(1 / job_size)) + 's'

    # Generate YAML file for job request
    yaml_content = generate_yaml(i + 1, gpu_required, sleep_time)

    # Write YAML content to file
    yaml_filename = f"job-{i + 1}.yaml"
    with open(yaml_filename, "w") as file:
        file.write(yaml_content)

    # Apply Kubernetes YAML file
    subprocess.run(["kubectl", "apply", "-f", f"job-{i + 1}.yaml"])
    print("Job YAML applied")

    # Print job details
    print(f"Job request {i + 1}: GPU(s) required: {gpu_required}")
    print("Job size is " + sleep_time)

    # Delete YAML file
    os.remove(yaml_filename)
    print(f"YAML file {yaml_filename} deleted\n")

num_pod_per_job = 1
# Check for completion of all jobs
job_status = check_job_status(num_pod_per_job)
while job_status < total_jobs:
    print("Number of completed jobs is:", job_status, "and the goal is:", total_jobs)
    time.sleep(10)
    job_status = check_job_status(num_pod_per_job)

# Run kubectl command and save output to file
print("Sending output...")
generate_output(output_file)


print("All job requests processed.")
