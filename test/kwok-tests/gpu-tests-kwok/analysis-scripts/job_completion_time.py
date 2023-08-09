import subprocess
import json
from datetime import datetime
import matplotlib.pyplot as plt

# Run kubectl command to get job information in JSON format
kubectl_command = ["kubectl", "get", "jobs", "-o", "json"]
result = subprocess.run(kubectl_command, capture_output=True, text=True)
job_data = result.stdout

# Load the JSON data into a dictionary
jobs_info = json.loads(job_data)

# Function to calculate job duration
def calculate_duration(start_time, completion_time):
    start_datetime = datetime.strptime(start_time, "%Y-%m-%dT%H:%M:%SZ")
    completion_datetime = datetime.strptime(completion_time, "%Y-%m-%dT%H:%M:%SZ")
    return completion_datetime - start_datetime

# Lists to store job details
job_details = []

# Process each job
for job_index, job_info in enumerate(jobs_info.get("items", [])):
    completion_time = job_info["status"].get("completionTime")
    creation_timestamp = job_info["metadata"]["creationTimestamp"]
    gpu_needed = job_info["spec"]["template"]["spec"]["containers"][0]["resources"]["limits"].get("nvidia.com/gpu", "")
    sleep_time = job_info["spec"]["template"]["spec"]["containers"][0]["args"][1]
    
    if completion_time:
        job_duration = calculate_duration(creation_timestamp, completion_time)
        job_details.append((job_index + 1, creation_timestamp, completion_time, gpu_needed, sleep_time, job_duration.total_seconds()))

# Save job details to output file
output_file = "job_details.txt"
with open(output_file, "w") as file:
    file.write("Job Index\tCreation Time\tCompletion Time\tGPUs Requested\tSleep Time\tJob Duration (seconds)\n")
    for details in job_details:
        file.write("\t".join(map(str, details)) + "\n")

# Create job indices for x-axis
job_indices = [details[0] for details in job_details]
job_durations = [details[5] for details in job_details]

# Plotting the bar plot
plt.bar(job_indices, job_durations, color='blue')
plt.xlabel('Job Index')
plt.ylabel('Job Duration (seconds)')
plt.title('Job Durations vs Job Indices')
plt.xticks(job_indices)
plt.tight_layout()

# Save and display the plot
output_plot_filename = "job_durations_plot.png"
plt.savefig(output_plot_filename)
plt.show()

# Calculate the average job completion time
total_completion_time = sum(details[5] for details in job_details if details[5] is not None)
average_completion_time = total_completion_time / len(job_details)

# Print average job completion time
print("Average Job Completion Time:", average_completion_time, "seconds")

print("Job details saved in", output_file)
print("Bar plot saved as", output_plot_filename)
