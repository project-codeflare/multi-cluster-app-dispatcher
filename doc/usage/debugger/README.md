 ## Local debugging with VSCode
 Steps outlining how to run MCAD locally from your IDE / debugger.
 - Ensure you are authenticated to your Kubernetes/OpenShift Cluster.
 - Copy the [launch.json.template](https://github.com/project-codeflare/multi-cluster-app-dispatcher/tree/main/doc/usage/debugger/launch.json.example) file and save it as `launch.json` in the `.vscode` folder in the root of your project.
 This can be done by running the following commands in the root of your project:
 ```bash
 mkdir --parents .vscode
 cp --interactive doc/usage/debugger/launch.json.template .vscode/launch.json
 ```
 - In the `launch.json` file, update the `args` sections with the path to your kubeconfig.
 - In VSCode on the activity bar click `Run and Debug` or `CTRL + SHIFT + D` to start a local debugging session of MCAD.

 ### Optional
 To set the desired log level set `--v`, `<desired_logging_level>` in the args sections of the `launch.json` file.
