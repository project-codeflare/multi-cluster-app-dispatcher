from datetime import datetime
import subprocess
import json
import csv
import matplotlib.pyplot as plt

def get_pods():
    cmd = "kubectl get pods -n default -o json"
    output = subprocess.check_output(cmd, shell=True)
    return json.loads(output)

def calculate_scheduling_latency(pod):
    creation_timestamp = pod['metadata']['creationTimestamp']
    pod_scheduled_transition_time = next(
        (cond['lastTransitionTime'] for cond in pod['status']['conditions'] if cond['type'] == 'PodScheduled'),
        None
    )

    if creation_timestamp and pod_scheduled_transition_time:
        creation_datetime = datetime.strptime(creation_timestamp, "%Y-%m-%dT%H:%M:%SZ")
        pod_scheduled_datetime = datetime.strptime(pod_scheduled_transition_time, "%Y-%m-%dT%H:%M:%SZ")
        scheduling_latency = (pod_scheduled_datetime - creation_datetime).total_seconds()
        return scheduling_latency
    else:
        return None

def get_pod_time_info(pod):
    if pod['status']['containerStatuses']:
        container_status = pod['status']['containerStatuses'][0]
        start_time = container_status['state'].get('terminated', {}).get('startedAt')
        finish_time = container_status['state'].get('terminated', {}).get('finishedAt')
        if start_time:
            start_datetime = datetime.strptime(start_time, "%Y-%m-%dT%H:%M:%SZ")
        else:
            start_datetime = None
        if finish_time:
            finish_datetime = datetime.strptime(finish_time, "%Y-%m-%dT%H:%M:%SZ")
        else:
            finish_datetime = None
        return start_datetime, finish_datetime
    return None, None

def main():
    pods = get_pods()

    scheduling_data = []
    scheduling_latencies = []
    for pod in pods['items']:
        pod_name = pod['metadata']['name']
        creation_timestamp = pod['metadata']['creationTimestamp']
        pod_scheduled_transition_time = next(
            (cond['lastTransitionTime'] for cond in pod['status']['conditions'] if cond['type'] == 'PodScheduled'),
            None
        )
        pod_start_time, pod_finish_time = get_pod_time_info(pod)
        if pod_scheduled_transition_time and pod_start_time and pod_finish_time:
            scheduling_latency = calculate_scheduling_latency(pod)
            scheduling_latencies.append(scheduling_latency)
            scheduling_data.append((pod_name, creation_timestamp, pod_scheduled_transition_time, pod_start_time, pod_finish_time, scheduling_latency))

    if scheduling_data:
        with open("scheduling_data.csv", mode="w", newline='') as csvfile:
            writer = csv.writer(csvfile)
            writer.writerow(["Pod Name", "Creation Time", "Scheduled Time", "Start Time", "Finish Time", "Scheduling Latency (seconds)"])
            for data in scheduling_data:
                writer.writerow(data)
        print("Scheduling data saved to scheduling_data.csv")
    else:
        print("No scheduling latency data available.")

    if scheduling_latencies:
        average_latency = sum(scheduling_latencies) / len(scheduling_latencies)
        print(f"Average Scheduling Latency: {average_latency:.2f} seconds")

        # Create a bar plot of scheduling latencies
        plt.figure(figsize=(10, 6))
        plt.bar(range(len(scheduling_latencies)), scheduling_latencies)
        plt.xlabel("Pod Index")
        plt.ylabel("Scheduling Latency (seconds)")
        plt.title("Scheduling Latencies for Pods")
        plt.tight_layout()
        plt.show()
    else:
        print("No scheduling latency data available.")


if __name__ == "__main__":
    main()
