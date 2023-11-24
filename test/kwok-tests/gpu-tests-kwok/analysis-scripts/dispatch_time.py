import subprocess
import json
import csv
from datetime import datetime
import matplotlib.pyplot as plt
import pandas as pd

def get_appwrappers():
    cmd = "kubectl get appwrappers -o json"
    output = subprocess.check_output(cmd, shell=True)
    return json.loads(output)

def extract_controller_dispatch_times(appwrappers, output_file):
    dispatch_times = []
    appwrapper_names = []
    controller_first_timestamps = []
    with open(output_file, mode='w', newline='') as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(["AppWrapper", "Controller First Timestamp", "Dispatch Time", "Average Dispatch Time"])

        for item in appwrappers['items']:
            appwrapper_name = item.get('metadata', {}).get('name')
            controller_first_timestamp = item.get('status', {}).get('controllerfirsttimestamp', None)

            conditions = item.get('status', {}).get('conditions', [])
            dispatch_time = None
            for condition in conditions:
                if condition.get('type', '') == 'Dispatched':
                    dispatch_time = condition.get('lastTransitionMicroTime', None)
                    break

            dispatch_datetime = datetime.strptime(dispatch_time, "%Y-%m-%dT%H:%M:%S.%fZ") if dispatch_time else None
            controller_first_datetime = datetime.strptime(controller_first_timestamp, "%Y-%m-%dT%H:%M:%S.%fZ") if controller_first_timestamp else None

            if dispatch_datetime and controller_first_datetime:
                average_dispatch_time = (dispatch_datetime - controller_first_datetime).total_seconds()
                dispatch_times.append(average_dispatch_time)
                appwrapper_names.append(appwrapper_name)
                controller_first_timestamps.append(controller_first_timestamp)

            writer.writerow([appwrapper_name, controller_first_timestamp, dispatch_time, average_dispatch_time])

    if dispatch_times:
        average_dispatch_time_overall = sum(dispatch_times) / len(dispatch_times)
        print(f"Average Dispatch Time Across All AppWrappers: {average_dispatch_time_overall:.2f} seconds")
    else:
        print("No data available to calculate average dispatch time.")

    # Sort the lists based on the controller first timestamp
    sorted_data = sorted(zip(appwrapper_names, dispatch_times, controller_first_timestamps),
                         key=lambda x: x[2])
    _, sorted_dispatch_times, _ = zip(*sorted_data)

    # Plot the dispatch time as a bar chart
    if sorted_dispatch_times:
        plt.figure(figsize=(12, 6))
        plt.bar(range(len(sorted_dispatch_times)), sorted_dispatch_times)
        plt.xlabel('Index')
        plt.ylabel('Dispatch Time (seconds)')
        plt.title('Dispatch Time vs. Index')
        plt.xticks(range(len(sorted_dispatch_times)), rotation=45, ha='right')
        plt.tight_layout()
        plt.show()

def main():
    appwrappers = get_appwrappers()
    output_file = "appwrapper_times.csv"
    extract_controller_dispatch_times(appwrappers, output_file)

if __name__ == "__main__":
    main()
