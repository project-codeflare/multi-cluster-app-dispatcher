# Simulation Analysis:


## 1. Kubernetes Scheduling Latency Analyzer

The `scheduling_latency.py` is a Python script that retrieves scheduling latency data from Kubernetes pods and analyzes it to provide insights into the scheduling performance of the cluster. The script collects information about pod creation, scheduling, start, and finish times, calculates scheduling latency, and generates visualizations for better understanding.

#### Configuration supported
nomcad, nomcad+cosched, mcad, mcad+cosched

#### Program Workflow

1. The script uses the `kubectl get pods` command to retrieve pod information in JSON format.
2. It calculates the scheduling latency for each pod by comparing the pod's creation time with its scheduled time.
3. The script extracts pod start and finish times and calculates the total time a pod takes to start and finish.
4. Scheduling latency data is saved to a CSV file named `scheduling_data.csv`.
5. If scheduling latency data is available, the script calculates the average scheduling latency.
6. The script generates a bar plot using `matplotlib` to visualize the distribution of scheduling latencies.

#### How to Run

1. Ensure you have Python 3.x and `kubectl` properly configured.
2. Run the script using: `python3 scheduling_latency.py`.

#### Output

- The script saves scheduling data to a CSV file named `scheduling_data.csv`.
- If scheduling latency data is available, the average scheduling latency is displayed.
- A bar plot displaying scheduling latencies is generated and shown using `matplotlib`.



## 2. Job Completion Time

The `job_completion_time.py` script retrieves job information from Kubernetes using the `kubectl` command, calculates the duration of each job, and generates a bar plot to visualize job durations. The script also calculates the average job completion time and saves the job details and the bar plot to files.

#### Configuration supported
nomcad, nomcad+cosched, mcad, mcad+cosched

#### Script Workflow

1. The script runs the `kubectl get jobs -o json` command to obtain job information in JSON format.
2. A function `calculate_duration` calculates the duration of each job.
4. Job details such as index, creation time, completion time, GPUs requested, sleep time, and job duration are extracted and stored.
5. Job details are saved to a text file named `job_details.txt`.
6. A bar plot is generated using `matplotlib` to visualize job durations.
7. The bar plot is saved as an image file named `job_durations_plot.png`.
8. The script calculates the average job completion time and displays it.

Run the script using: 
```
python job_completion_time.py
```

#### Output

- The script saves job details to a text file named `job_details.txt`.
- A bar plot displaying job durations is generated and saved as an image file named `job_durations_plot.png`.
- The script calculates and displays the average job completion time.


## 3. Pending Pods

The `plot_pending_pods.py` script aids in comparing the impact of MCAD and Non-MCAD approaches on the number of pending pods.
The script reads data from two files containing submission times and the pending pod counts at the job submission time for two different scenarios: MCAD  and NO-MCAD. The files are obtained from `run_sim.py` experiment run, hence set the appropriate name for files using `--pending-pod` argument for `run_sim.py`. The script then creates a plot to visualize the variation of pending pods over submission time for both scenarios. The plot provides insights into the efficiency of job scheduling and resource allocation in the two scenarios.

#### Configuration supported
nomcad, nomcad+cosched, mcad, mcad+cosched

An example run of this script is as follows:
```
python plot_pending_pods.py --mcad-file mcad_data.txt --nomcad-file nomcad_data.txt
```


## 4. Dispatch Time

The `dispatch_time.py` script interacts with Kubernetes to retrieve information about AppWrappers and their dispatch times. It calculates the average dispatch time across all AppWrappers and generates a bar chart to visualize the distribution of dispatch times.
The script provides insights into the efficiency of dispatch times for AppWrappers in a Kubernetes environment. Dispatch times can affect the responsiveness of scheduling and resource allocation.

#### Configuration supported
mcad, mcad+cosched

### Script Workflow

1. The script defines a function `get_appwrappers` to retrieve AppWrapper information using the `kubectl get appwrappers -o json` command.
2. Another function `extract_controller_dispatch_times` processes AppWrapper data, extracts dispatch times, and calculates the average dispatch time.
3. AppWrapper names, controller first timestamps, dispatch times, and average dispatch times are collected and saved to a CSV file.
4. If dispatch times are available, the script calculates and displays the average dispatch time across all AppWrappers.
5. The collected data is sorted based on controller first timestamps.
6. A bar chart is generated using `matplotlib` to visualize the distribution of dispatch times.
7. The plot displays dispatch times against indices.

An example run:
```
python dispatch_time.py
```

### Output

- The script generates a CSV file named `appwrapper_times.csv` containing AppWrapper names, controller first timestamps, AW dispatch time, and dispatch interval.
- If dispatch intervals are available, the script displays the average dispatch interval across all AppWrappers.
- A bar chart is generated and displayed, showing the distribution of dispatch times for AppWrappers.


## 5. MCAD Response Time

The `mcad_response_time.py` script utilizes Kubernetes data to calculate and visualize response times and job completion times in an MCAD (Multi-Cluster Application Deployment) system. The script extracts completion times and creation times for job and AppWrapper. The script computes job completion time as the difference between job creation time and job completion time. The script also computes AW response time as the difference between AW creation time and AW completion time, and generates a scatter plot to visualize the relationship between response times and job completion times.

#### Configuration supported
mcad, mcad+cosched


### Script Workflow

1. The script runs `kubectl` commands to retrieve job and appwrapper information in JSON format.
2. Job completion times, job creation time, appwrapper controller first timestamps (creation time) and appwrapper completion times are extracted from the data.
4. Differences between mcad controller first timestamps and appwrapper completion times are calculated to obtain response times.
5. Differences between job completion times and job creation times are calculated to obtain job completion times.
6. The average response time and average job completion time are calculated.
7. A scatter plot is generated using `matplotlib` to visualize the relationship between response times and job completion times.

An example run:
```
python mcad_response_time.py
```

### Output
The script calculates and displays the average response time and average job completion time in the MCAD system.
- A line plot is generated and displayed, offering a visual representation of job completion times and response times.
- The plot aids in understanding the distribution and trends in response and completion times.



## 6. GPU Utilization

The `gpu_usage.py` script monitors and visualizes the total GPU resource requests and limits over time for nodes labeled with "type=kwok" in a Kubernetes cluster. This script utilizes the `kubectl` command-line tool to gather GPU resource information from the specified nodes, providing real-time insights into GPU utilization.

#### Configuration supported
nomcad, nomcad+cosched, mcad, mcad+cosched

### Script Workflow

1. The script initiates an infinite loop to continuously monitor GPU resource usage.
2. It retrieves the names of all nodes labeled with "type=kwok" using the `kubectl` command.
3. For each Kwok node, it extracts the total GPU resource requests and limits by calling the `get_gpu_resource_info()` function.
4. The script records the total GPU resource requests and limits.
5. After each iteration, the script waits for 20 seconds before checking again.
6. If the script is interrupted by the user (keyboard interrupt), it generates an output text file named `gpu_resource_records.txt` to store the collected records.
7. If sufficient records are available, a line plot is created using `matplotlib`, displaying the variation in total GPU resource requests and limits over time.

Execute the script with: 
```
python gpu_usage.py
```

### Output
- The script continuously records the total GPU resource requests and limits over time, storing them in the `gpu_resource_records.txt` file.
- If an adequate number of records is collected, a line plot is generated, illustrating the change in total GPU resource requests and limits over time.
- The plot offers insights into GPU utilization trends for the designated nodes.


