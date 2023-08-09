import subprocess
import json
import matplotlib.pyplot as plt
import numpy as np
from datetime import datetime

# Run kubectl command to get job and appwrapper information in JSON format
kubectl_command = ["kubectl", "get", "job,appwrapper", "-o", "json"]
result = subprocess.run(kubectl_command, capture_output=True, text=True)
data = result.stdout

# Load the JSON data into dictionaries
info = json.loads(data)

# Initialize lists to store data
timestamps = {
    "controller_first": [],
    "appwrapper_creation": [],
    "job_creation": [],
    "job_completion": [],
    "appwrapper_completion": [],
}
completion_times = []
response_times = []

output_lines = []

# Process the data
for item in info.get("items", []):
    kind = item.get("kind")
    metadata = item.get("metadata", {})
    status = item.get("status", {})

    if kind == "Job":
        completion_time = status.get("completionTime")
        creation_time = metadata.get("creationTimestamp")

        if completion_time and creation_time:
            completion_time = datetime.strptime(completion_time, "%Y-%m-%dT%H:%M:%SZ")
            creation_time = datetime.strptime(creation_time, "%Y-%m-%dT%H:%M:%SZ")
            timestamps["job_creation"].append(creation_time)
            timestamps["job_completion"].append(completion_time)
            completion_times.append(completion_time)

    elif kind == "AppWrapper":
        controller_first = status.get("controllerfirsttimestamp")
        creation_time = metadata.get("creationTimestamp")
        conditions = status.get("conditions", [])
        appwrapper_completion_time = None

        for condition in conditions:
            if condition.get("type") == "Completed":
                appwrapper_completion_time = condition.get("lastTransitionMicroTime")
                break

        if controller_first and creation_time and appwrapper_completion_time:
            controller_first = datetime.strptime(controller_first, "%Y-%m-%dT%H:%M:%S.%fZ")
            creation_time = datetime.strptime(creation_time, "%Y-%m-%dT%H:%M:%SZ")
            appwrapper_completion_time = datetime.strptime(appwrapper_completion_time, "%Y-%m-%dT%H:%M:%S.%fZ")
            timestamps["controller_first"].append(controller_first)
            timestamps["appwrapper_creation"].append(creation_time)
            timestamps["appwrapper_completion"].append(appwrapper_completion_time)

            response_time = (appwrapper_completion_time - controller_first).total_seconds()
            response_times.append(response_time)

# Calculate the average response time
average_response_time = np.mean(response_times)
print(f"Average Response Time: {average_response_time:.2f} seconds")

# Calculate the average job completion time
average_job_completion_time = np.mean([(completion - creation).total_seconds() for completion, creation in zip(completion_times, timestamps["job_creation"])])
print(f"Average Job Completion Time: {average_job_completion_time:.2f} seconds")

# Write output lines to a text file
output_lines = [
    "AW Controller First, AW Creation, Job Creation, Job Completion, AW Completion, Job Completion Time, Response Time\n"
]
with open("output.txt", "w") as output_file:
    for i in range(len(completion_times)):
        output_line = (
            f"{timestamps['controller_first'][i]}, "
            f"{timestamps['appwrapper_creation'][i]}, "
            f"{timestamps['job_creation'][i]}, "
            f"{timestamps['job_completion'][i]}, "
            f"{timestamps['appwrapper_completion'][i]}, "
            f"{(timestamps['job_completion'][i] - timestamps['job_creation'][i]).total_seconds():.2f}, "
            f"{response_times[i]:.2f}\n"
        )
        output_lines.append(output_line)
    output_file.writelines(output_lines)

# Plot the line plot for job completion times and response times
plt.figure(figsize=(12, 6))
plt.plot(range(len(completion_times)), [(completion - creation).total_seconds() for completion, creation in zip(completion_times, timestamps["job_creation"])], marker='o', color='b', alpha=0.5, label='Job Completion')
plt.plot(range(len(completion_times)), response_times, marker='*', color='r', alpha=0.5, label='Response Time')
plt.xlabel("Index")
plt.ylabel("Time (seconds)")
plt.title("Line Plot of Response Time and Job Completion Time")
plt.legend()
plt.tight_layout()
plt.show()