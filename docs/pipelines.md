# pipelines

## pipeline-runner/

This pipeline is a simple build and deploy process where all build is happening using Dockerfile.

The steps are:

- Apply `radixconfig.yaml` file to ensure the application is up-to-date.
- Read current radixconfig back from K8s API to use in the next pipeline jobs
- Build using `docker build -t Dockerfile .` for each component specified in `radixconfig.yaml`
- Push the built images to container registry
- Deploy each component
