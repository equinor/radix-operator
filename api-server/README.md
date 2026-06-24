
# Radix API Server

The Radix API Server is an HTTP server for accessing functionality on the [Radix](https://www.radix.equinor.com) platform. This document is for Radix developers, or anyone interested in poking around.

## Purpose

The Radix API is meant to be the single point of entry for platform users to the platform (through e.g. the Web Console). Users should not be able to access the Kubernetes API directly; therefore the Radix API limits and customises what platform users are able to do.

## Security

Authentication and authorisation are performed through an HTTP bearer token, which is relayed to the Kubernetes API. The Kubernetes AAD integration then performs its authentication and resource authorisation checks, and the result is relayed to the user.

### Running locally

The following env vars are needed. Useful default values in brackets.

- `RADIX_CONTAINER_REGISTRY` - (`radixdev.azurecr.io`)
- `PROMETHEUS_URL` - `http://localhost:9091` use this to get Prometheus metrics running the following command (the local port 9090 is used by the API server `/metrics` endpoint, in-cluster URL is <http://prometheus-operator-prometheus.monitor.svc.cluster.local:9090>):

  ```
  kubectl -n monitor port-forward svc/prometheus-operator-prometheus 9091:9090
  ```

If you are using VSCode, there is a convenient launch configuration in `.vscode`.
