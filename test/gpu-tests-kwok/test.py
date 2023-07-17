import random
import time
import subprocess
from datetime import datetime
import os

# Parameters
mean_arrival = 50  # mean arrival time for job requests in seconds
total_jobs = 100  # Total number of job requests to generate
job_size = 60 # mean sleep time of container in seconds

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
              nvidia.com/gpu: {gpu_required}
            requests:
              nvidia.com/gpu: {gpu_required}
      restartPolicy: Never
"""
    return yaml_template

# set the seed
random.seed(datetime.now().timestamp())

# Simulate job requests
for i in range(total_jobs):
    # Simulate job request arrival based on Poisson process
    inter_arrival_time = random.expovariate(1/mean_arrival)
    print("Next arrival after " + str(inter_arrival_time) + " seconds \n")
    time.sleep(inter_arrival_time)

    # Generate job request
    gpu_required = generate_job_request()

    # generate job size taken from exponential distribution
    sleep_time = str(random.expovariate(1/job_size)) + 's'

    # Generate YAML file for job request
    yaml_content = generate_yaml(i+1, gpu_required, sleep_time)

    # Write YAML content to file
    yaml_filename = f"job-{i+1}.yaml"
    with open(yaml_filename, "w") as file:
        file.write(yaml_content)

    # Apply Kubernetes YAML file
    subprocess.run(["kubectl", "apply", "-f", f"job-{i+1}.yaml"])
    print("Job YAML applied")

    # Print job details
    print(f"Job request {i+1}: GPU(s) required: {gpu_required}")
    print("Job size is " + sleep_time)

    # Delete YAML file
    os.remove(yaml_filename)
    print(f"YAML file {yaml_filename} deleted\n")

print("All job requests processed.")
