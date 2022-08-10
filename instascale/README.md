## Instascale controller

# High Level Steps
- User submits Multi GPU job(s)
- Job(s) lands in MCAD queue
- When resources are not available it triggers scaling i.e. calls instascale
- Instascale looks at resource requests specified by the user and matches those with the desired Machineset(s) to get nodes.
- After instascaling, when aggregate resources are available to run the job MCAD dispatches the job.
- When job completes, resources obtained for the job are released.

# Usage
- To build locally : `make run`
- To build and release a docker image for controller : `make IMG=asm582/instascale docker-build docker-push`
- To deploy on kubernetes cluster use deployments for now

# Testing

TBD