import subprocess
import json
import matplotlib.pyplot as plt
import numpy as np
import argparse
from datetime import datetime

# Run kubectl command to get job information in JSON format
kubectl_job_command = ["kubectl", "get", "job", "-o", "json"]
result_job = subprocess.run(kubectl_job_command, capture_output=True, text=True)
job_data = result_job.stdout

# Run kubectl command to get appwrapper information in JSON format
kubectl_appwrapper_command = ["kubectl", "get", "appwrapper", "-o", "json"]
result_appwrapper = subprocess.run(kubectl_appwrapper_command, capture_output=True, text=True)
appwrapper_data = result_appwrapper.stdout

# Load the JSON data into dictionaries
jobs_info = json.loads(job_data)
appwrapper_info = json.loads(appwrapper_data)

# Extract completion times and controller first timestamps from jobs_info and appwrapper_info
completion_times = []
controller_first_timestamps = []
creation_times = []

for job_info, appwrapper_info in zip(jobs_info.get("items", []), appwrapper_info.get("items", [])):
    completion_time = job_info["status"].get("completionTime")
    controller_first_timestamp = appwrapper_info["status"].get("controllerfirsttimestamp")
    creation_time = job_info["metadata"]["creationTimestamp"]
    
    if completion_time and controller_first_timestamp:
        completion_time = datetime.strptime(completion_time, "%Y-%m-%dT%H:%M:%SZ")  # Updated format string
        controller_first_timestamp = datetime.strptime(controller_first_timestamp, "%Y-%m-%dT%H:%M:%S.%fZ")  # Updated format string
        creation_time = datetime.strptime(creation_time, "%Y-%m-%dT%H:%M:%SZ")
        completion_times.append(completion_time)
        controller_first_timestamps.append(controller_first_timestamp)
        creation_times.append(creation_time)

# Compute the differences between controller first timestamp and completion time
time_differences = [(completion - controller_first).total_seconds() for completion, controller_first in zip(completion_times, controller_first_timestamps)]

# Compute the job completion time
job_completion = [(completion - creation).total_seconds() for completion, creation in zip(completion_times, creation_times)]

# Calculate the average response time
average_response_time = np.mean(time_differences)
print(f"Average Response Time of a Job in an MCAD system: {average_response_time:.2f} seconds")

# Calculate the average job completion time
average_completion_time = np.mean(job_completion)
print(f"Average Job Completion Time in an MCAD system: {average_completion_time:.2f} seconds")

# Plot the scatter plot for job completion times and response times
plt.figure(figsize=(12, 6))

# Line plot for job completion times
plt.plot(range(len(job_completion)), job_completion, marker='o', color='b', alpha=0.5, label='Job Completion')

# Line plot for response times
plt.plot(range(len(time_differences)), time_differences, marker='*', color='r', alpha=0.5, label='Response Time')
plt.xlabel("Index")
plt.ylabel("Time (seconds)")
plt.title("Scatter Plot of Response Time and Job Completion Time")
plt.legend()
plt.tight_layout()
plt.show()



