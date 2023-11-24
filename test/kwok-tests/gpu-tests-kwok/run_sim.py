import random
import time
import subprocess
from datetime import datetime
import os
import argparse
import json
import yaml
import random

# Function to check job status
def check_job_status(num_pod_per_job):
    command = ["kubectl", "get", "jobs", "-n", "default", "--no-headers", "--field-selector", f"status.successful={num_pod_per_job}"]
    result = subprocess.run(command, capture_output=True, text=True)
    output = result.stdout.strip()
    completed_jobs = len(output.splitlines())
    return completed_jobs

# Function to write output to file
def generate_output(output_file, data, mode="nomcad"):
    if mode == "mcad":
        column_names = ["NAME", "MCAD_CONTROLLER_FIRST_TIMESTAMP", "JOB_CREATION_TIME", "JOB_COMPLETION_TIME", "GPU_NEEDED", "SLEEP_TIME"]
    elif mode == "nomcad":
        column_names = ["NAME", "CREATION_TIME", "COMPLETION_TIME", "GPU_NEEDED", "SLEEP_TIME"]
    else:
        raise ValueError("Invalid mode. Use 'mcad' or 'nomcad'.")

    # Extract and format desired columns
    job_results = []
    job_results.append("\t".join(column_names))  # Add column names as the first row
    for item in data.get("items", []):
        name = item["metadata"]["name"]
        creation_timestamp = item["metadata"]["creationTimestamp"]
        completion_time = item["status"].get("completionTime", "")
        gpu_needed = item["spec"]["template"]["spec"]["containers"][0]["resources"]["limits"].get("nvidia.com/gpu", "")
        sleep_time = item["spec"]["template"]["spec"]["containers"][0]["args"][1]

        if mode == "mcad":
            # Extract controller first timestamp from AppWrapper output
            appwrapper_output_command = subprocess.run(["kubectl", "get", "appwrapper", name, "-o", "yaml"], capture_output=True, text=True)
            appwrapper_output = appwrapper_output_command.stdout.strip()
            appwrapper_data = yaml.safe_load(appwrapper_output)
            controller_first_timestamp = appwrapper_data.get("status", {}).get("controllerfirsttimestamp", "")

            # Format controller first timestamp as YYYY-MM-DDTHH:MM:SS
            formatted_controller_first_timestamp = datetime.strptime(controller_first_timestamp, "%Y-%m-%dT%H:%M:%S.%fZ").strftime("%Y-%m-%dT%H:%M:%SZ")

            # Append formatted row to results list
            job_results.append(f"{name}\t{formatted_controller_first_timestamp}\t{creation_timestamp}\t{completion_time}\t{gpu_needed}\t{sleep_time}")
        elif mode == "nomcad":
            # Append formatted row to results list
            job_results.append(f"{name}\t{creation_timestamp}\t{completion_time}\t{gpu_needed}\t{sleep_time}")

    # Join results with newlines
    formatted_output = "\n".join(job_results)

    # Write output to file
    with open(output_file, "w") as file:
        file.write(formatted_output)


# Function to write pending pod count and submission time to a file
def write_pending_pods_to_file(output_file, submission_times, pending_pods_counts):
    with open(output_file, "w") as file:
        file.write("Submission Time\tPending Pods\n")
        for time, count in zip(submission_times, pending_pods_counts):
            file.write(f"{time}\t{count}\n")

# Function to generate job requests
def generate_job_request(gpu_options, probabilities):
    gpu_needed = random.choices(gpu_options, probabilities)[0]
    return gpu_needed

# Function to get the number of pending pods
def get_pending_pods():
    command = ["kubectl", "get", "pods", "-n", "default", "--field-selector", "spec.nodeName=", "--no-headers"]
    result = subprocess.run(command, capture_output=True, text=True)
    output = result.stdout.strip()
    pending_pods = len(output.splitlines())
    return pending_pods

def generate_mcadcosched_yaml(job_id, gpu_required, sleep_time, num_pod):
    yaml_template = f"""
apiVersion: mcad.ibm.com/v1beta1
kind: AppWrapper
metadata:
  name: mcadkwokcosched-gpu-job-{job_id}
  namespace: default
spec:
  resources:
    Items: []
    GenericItems:
      - replicas: 1
        generictemplate:
          apiVersion: scheduling.x-k8s.io/v1alpha1
          kind: PodGroup
          metadata:
            name: pg-{job_id}
            namespace: default
          spec:
            scheduleTimeoutSeconds: 10
            minMember: {num_pod}
      - replicas: 1
        generictemplate:
          apiVersion: batch/v1
          kind: Job
          metadata:
            name: mcadkwokcosched-gpu-job-{job_id}
            namespace: default
          spec:
            completions: {num_pod}          
            parallelism: {num_pod} 
            template:
              metadata:
                labels:
                  app: mcadkwokcosched-gpu-job-{job_id}
                  appwrapper.mcad.ibm.com: mcadkwokcosched-gpu-job-{job_id}
                  scheduling.x-k8s.io/pod-group: pg-{job_id}
              spec:
                schedulerName: scheduler-plugins-scheduler
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
                    name: mcadkwokcosched-gpu-job-{job_id}
                    image: registry.k8s.io/pause:3.6
                    resources:
                      requests:
                        cpu: 5m
                        memory: 20M
                        nvidia.com/gpu: {gpu_required}
                      limits:
                        cpu: 5m
                        memory: 20M
                        nvidia.com/gpu: {gpu_required}
                restartPolicy: Never
"""
    return yaml_template


def generate_nomcadcosched_yaml(job_id, gpu_required, sleep_time, num_pod):
    yaml_template = f"""
apiVersion: scheduling.x-k8s.io/v1alpha1
kind: PodGroup
metadata:
  name: pg-{job_id}
spec:
  scheduleTimeoutSeconds: 10
  minMember: {num_pod}
---
apiVersion: batch/v1
kind: Job
metadata:
  namespace: default
  name: nomcadkwok-job-{job_id}
spec:
  parallelism: {num_pod}
  completions: {num_pod}
  template:
    metadata:
      namespace: default
      labels:
        scheduling.x-k8s.io/pod-group: pg-{job_id}
    spec:
      schedulerName: scheduler-plugins-scheduler
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
        name: nomcadkwok-job-{job_id}
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

# Function to generate YAML file for mcad job request
def generate_mcad_yaml(job_id, gpu_required, sleep_time):
    yaml_template = f"""
apiVersion: mcad.ibm.com/v1beta1
kind: AppWrapper
metadata:
  name: mcadkwok-gpu-job-{job_id}
  namespace: default
spec:
  resources:
    Items: []
    GenericItems:
    - replicas: 1
      completionstatus: Complete
      generictemplate:
        apiVersion: batch/v1
        kind: Job
        metadata:
          namespace: default
          name: mcadkwok-gpu-job-{job_id}
        spec:
          parallelism: 1
          completions: 1
          template:
            metadata:
              namespace: default
              labels:
                appwrapper.mcad.ibm.com: mcadkwok-gpu-job-{job_id}
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
                name: mcadkwok-gpu-job-{job_id}
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

# Function to generate YAML file for nomcad job request
def generate_nomcad_yaml(job_id, gpu_required, sleep_time):
    yaml_template = f"""
apiVersion: batch/v1
kind: Job
metadata:
  namespace: default
  name: nomcadkwok-job-{job_id}
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
        name: nomcadkwok-job-{job_id}
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
parser.add_argument("--mean-arrival", type=float, default=50, help="Mean arrival time for job requests in seconds")
parser.add_argument("--total-jobs", type=int, default=100, help="Total number of job requests to generate")
parser.add_argument("--job-size", type=int, default=60, help="Mean sleep time of container in seconds")
parser.add_argument("--output-file", type=str, default="job_results.txt", help="Output file to store job results")
parser.add_argument("--pending-pod", type=str, default="pending_pods.txt", help="Output file to store number of pending pods")
parser.add_argument("--num-pod", type=int, default=1, help="Number of pods per job")
parser.add_argument("--gpu-options", type=int, nargs='+', default=[2, 4, 6, 8], help="Options for GPU requirements")
parser.add_argument("--probabilities", type=float, nargs='+', default=[0.25, 0.25, 0.25, 0.25], help="Probabilities for GPU requirements")
parser.add_argument("--mode", type=str, default="mcad", help="Mode: 'mcad' or 'nomcad'")
parser.add_argument("--israndom", type=lambda x: x.lower() in ['true', '1', 'yes'], default=True, help="Use random.expovariate for job request arrival and job size")
parser.add_argument("--usecosched", type=lambda x: x.lower() in ['true', '1', 'yes'], default=False, help="If true, then use coscheduler with number of pods specified by --num-pod argument")
args = parser.parse_args()

# Set random seed
random.seed(datetime.now().timestamp())

# Parameters from command line arguments
mean_arrival = args.mean_arrival
total_jobs = args.total_jobs
job_size = args.job_size
output_file = args.output_file
output_file2 = args.pending_pod
num_pod_per_job = args.num_pod
gpu_options = args.gpu_options
probabilities = args.probabilities
mode = args.mode

if sum(probabilities) != 1:
    raise ValueError("Probabilities for GPU requirements should add up to 1")

# Lists to store job submission times and pending pod counts
submission_times = []
pending_pods_counts = []

# get the start time of the experiment
start = datetime.now().timestamp()

# Simulate job requests
for i in range(total_jobs):
    # Simulate job request arrival based on the mode
    if args.israndom:
        inter_arrival_time = random.expovariate(1 / mean_arrival)
    else:
        inter_arrival_time = mean_arrival
    print("Next arrival after " + str(inter_arrival_time) + " seconds \n")
    time.sleep(inter_arrival_time)

    # Generate job request
    gpu_required = generate_job_request(gpu_options, probabilities)

    # Generate job size taken from exponential distribution
    # sleep_time = str(job_size) + 's' #str(random.expovariate(1 / job_size)) + 's'
    if args.israndom:
        sleep_time = str(random.expovariate(1 / job_size)) + 's'
    else:
        sleep_time = str(job_size) + 's'

    # Generate YAML file for job request based on the mode
    if mode == "mcad":
        if args.usecosched:
            yaml_content = generate_mcadcosched_yaml(i + 1, gpu_required, sleep_time, num_pod_per_job)
        else:
            yaml_content = generate_mcad_yaml(i + 1, gpu_required, sleep_time)
    elif mode == "nomcad":
        if args.usecosched:
            yaml_content = generate_nomcadcosched_yaml(i + 1, gpu_required, sleep_time, num_pod_per_job)
        else:
            yaml_content = generate_nomcad_yaml(i + 1, gpu_required, sleep_time)
    else:
        raise ValueError("Invalid mode. Use 'mcad' or 'nomcad'.")

    # Write YAML content to file
    yaml_filename = f"job-{i + 1}.yaml"
    with open(yaml_filename, "w") as file:
        file.write(yaml_content)

    # Apply Kubernetes YAML file
    subprocess.run(["kubectl", "apply", "-f", f"job-{i + 1}.yaml"])
    print("Job YAML applied")

    # Get the number of pending pods
    pending_pods = get_pending_pods()
    submission_times.append(datetime.now().timestamp() - start)
    pending_pods_counts.append(pending_pods)

    # Print job details
    print(f"Job request {i + 1}: GPU(s) required: {gpu_required}")
    print("Job size is " + sleep_time)
    print(f"Number of pending pods: {pending_pods}")

    # Delete YAML file
    os.remove(yaml_filename)
    print(f"YAML file {yaml_filename} deleted\n")

# Check for completion of all jobs
job_status = check_job_status(num_pod_per_job)
while job_status < total_jobs:
    print("Number of completed jobs is:", job_status, "and the goal is:", total_jobs)
    time.sleep(10)
    job_status = check_job_status(num_pod_per_job)

# Run kubectl commands and save outputs to files
print("Sending output...")
generate_output(output_file, json.loads(subprocess.run(["kubectl", "get", "jobs", "-o", "json"], capture_output=True, text=True).stdout), mode)
write_pending_pods_to_file(output_file2, submission_times, pending_pods_counts)
print("All job requests processed.")
