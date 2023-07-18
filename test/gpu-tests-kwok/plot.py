import matplotlib.pyplot as plt
from datetime import datetime
import numpy as np

output_file = "job_results.txt"
# Read the job results from the output file
with open(output_file, "r") as file:
    lines = file.readlines()

# Extract completion times and creation times
completion_times = []
creation_times = []
for line in lines[1:]:
    values = line.split()
    completion_times.append(values[2])
    creation_times.append(values[1])

# Convert completion times and creation times to datetime objects
completion_times = [datetime.strptime(time, "%Y-%m-%dT%H:%M:%SZ") for time in completion_times]
creation_times = [datetime.strptime(time, "%Y-%m-%dT%H:%M:%SZ") for time in creation_times]

# Compute the differences between completion time and creation time
time_differences = [(completion - creation).total_seconds() for completion, creation in zip(completion_times, creation_times)]

# Sort the time differences in ascending order
sorted_time_differences = np.sort(time_differences)

# Compute the cumulative probability
cumulative_probability = np.arange(1, len(sorted_time_differences) + 1) / len(sorted_time_differences)

# Plot the CDF
plt.plot(sorted_time_differences, cumulative_probability, label="CDF")
plt.xlabel(" Response Time (seconds)")
plt.ylabel("Cumulative Probability")
plt.title("CDF of Response Time for a system without MCAD")
plt.legend()
plt.grid(True)
plt.show()
