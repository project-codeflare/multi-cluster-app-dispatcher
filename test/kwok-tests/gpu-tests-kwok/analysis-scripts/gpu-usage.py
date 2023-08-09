import subprocess
import re
import time
import matplotlib.pyplot as plt

def get_gpu_resource_info(node_name):
    # Run the kubectl describe node command
    cmd = f"kubectl describe node {node_name}"
    output = subprocess.check_output(cmd, shell=True, text=True)

    # Use regular expressions to extract the GPU resource information
    gpu_requests = re.search(r"nvidia\.com/gpu\s+(\d+)", output)
    gpu_limits = re.search(r"nvidia\.com/gpu\s+\d+\s+(\d+)", output)

    if gpu_requests and gpu_limits:
        gpu_requests = int(gpu_requests.group(1))
        gpu_limits = int(gpu_limits.group(1))
        return gpu_requests, gpu_limits
    else:
        return None, None

def get_all_kwok_nodes():
    # Run kubectl get nodes command with label selector type=kwok
    cmd = "kubectl get nodes -l type=kwok --no-headers -o custom-columns='NAME:.metadata.name'"
    output = subprocess.check_output(cmd, shell=True, text=True)
    return output.strip().split("\n")

if __name__ == "__main__":
    output_file = "gpu_resource_records.txt"
    records = []

    try:
        while True:
            kwok_nodes = get_all_kwok_nodes()

            if not kwok_nodes:
                print("No nodes with label selector type=kwok found.")
                break

            total_requests = 0
            total_limits = 0

            for node_name in kwok_nodes:
                requests, limits = get_gpu_resource_info(node_name)
                if requests is not None and limits is not None:
                    total_requests += requests
                    total_limits += limits

            records.append((total_requests, total_limits))

            # Wait for 30 seconds before checking again
            time.sleep(20)

    except KeyboardInterrupt:
        print("Script stopped by the user.")

        with open(output_file, "w") as f:
            for index, (requests, limits) in enumerate(records):
                f.write(f"Index: {index}, Total GPU Resource Requests: {requests}, Total GPU Resource Limits: {limits}\n")

        if len(records) > 1:
            # Extract the individual lists of total_requests and total_limits
            total_requests_list, total_limits_list = zip(*records)

            # Create the plot
            plt.plot(total_requests_list, label='Total GPU Resource Requests')
            plt.plot(total_limits_list, label='Total GPU Resource Limits')

            # Add labels and title
            plt.xlabel('Index')
            plt.ylabel('GPU Resource')
            plt.title('GPU Resource Requests and Limits over time')
            plt.legend()

            # Show the plot
            plt.show()

        else:
            print("Insufficient records to plot")

