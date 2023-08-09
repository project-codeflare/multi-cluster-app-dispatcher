
import matplotlib.pyplot as plt
import argparse

def read_data(file_path):
    submission_times = []
    pending_pods = []
    with open(file_path, "r") as file:
        lines = file.readlines()[1:]  # Skip the header line
        for line in lines:
            time, pods = line.strip().split("\t")
            submission_times.append(float(time))
            pending_pods.append(int(pods))
    return submission_times, pending_pods

# Parse command-line arguments
parser = argparse.ArgumentParser(description="Plot pending pods vs submission time for MCAD and NOMCAD.")
parser.add_argument("--mcad-file", type=str, default="mcad_data.txt", help="Path to the MCAD data file")
parser.add_argument("--nomcad-file", type=str, default="nomcad_data.txt", help="Path to the NOMCAD data file")
args = parser.parse_args()

# Read data from the files
mcad_submission_times, mcad_pending_pods = read_data(args.mcad_file)
nomcad_submission_times, nomcad_pending_pods = read_data(args.nomcad_file)

# Calculate the common x-axis limits
min_time = min(min(mcad_submission_times), min(nomcad_submission_times))
max_time = max(max(mcad_submission_times), max(nomcad_submission_times))

# Create the plot
plt.plot(mcad_submission_times, mcad_pending_pods, label="MCAD")
plt.plot(nomcad_submission_times, nomcad_pending_pods, label="NO-MCAD")

# Set labels and title
plt.xlabel("Submission Time")
plt.ylabel("Pending Pods")
plt.title("Pending Pods vs Submission Time")
plt.legend()

# # Set the same x-axis scale for both plots
plt.xlim(min_time, max_time)

# Show the plot
plt.show()

