# Brigade pipelines

## Generic-build.js

This pipeline is a simple build and deploy process where all build is happening using Dockerfile.

The steps are:

- Apply `radixconfig.yaml` file to ensure the application is up-to-date.
- Read current radixconfig back from K8s API to use in the next pipeline jobs
- Build using `docker build -t Dockerfile .` for each component specified in `radixconfig.yaml`
- Push the built images to container registry
- Deploy each component

The way we read the radixconfig is by use of `kubectl` - this is because the Brigade script is loaded on build start and does not have a direct connection to the radixconfig. So, in order to get the updated values we need to rely on retrieving it through a Brigade job. 
This is done by first executing `kubectl apply`, sleep for a set period to allow the Radix Operator to perform its updates and then `kubectl get` to retrieve the updated radixconfig and passing this back to the Brigade script by using Job output.