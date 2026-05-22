
# Radix API Server

The Radix API Server is an HTTP server for accessing functionality on the [Radix](https://www.radix.equinor.com) platform. This document is for Radix developers, or anyone interested in poking around.

## Purpose

The Radix API is meant to be the single point of entry for platform users to the platform (through e.g. the Web Console). Users should not be able to access the Kubernetes API directly; therefore the Radix API limits and customises what platform users are able to do.

## Security

Authentication and authorisation are performed through an HTTP bearer token, which is relayed to the Kubernetes API. The Kubernetes AAD integration then performs its authentication and resource authorisation checks, and the result is relayed to the user.

## Developing

You need Go installed. Make sure `GOPATH` and `GOROOT` are properly set up.

Also needed:

- [`go-swagger`](https://github.com/go-swagger/go-swagger) (install with `make bootstrap`.)
- [`golangci-lint`](https://golangci-lint.run/) (install with `make bootstrap`)
- [`gomock`](https://github.com/uber-go/mock) (install with `make bootstrap`)

Clone the repo into your `GOPATH` and run `go mod download`.

### Dependencies - go modules

Go modules are used for dependency management. See [link](https://blog.golang.org/using-go-modules) for information how to add, upgrade and remove dependencies. E.g. To update `radix-operator` dependency:

- list versions: `go list -m -versions github.com/equinor/radix-operator`
- update: `go get github.com/equinor/radix-operator@v1.3.1`

### Generating mocks
We use gomock to generate mocks used in unit test.
You need to regenerate mocks if you make changes to any of the interface types used by the application.
```
make mocks
```

### Running locally

The following env vars are needed. Useful default values in brackets.

- `RADIX_CONTAINER_REGISTRY` - (`radixdev.azurecr.io`)
- `PROMETHEUS_URL` - `http://localhost:9091` use this to get Prometheus metrics running the following command (the local port 9090 is used by the API server `/metrics` endpoint, in-cluster URL is http://prometheus-operator-prometheus.monitor.svc.cluster.local:9090): 
  ```
  kubectl -n monitor port-forward svc/prometheus-operator-prometheus 9091:9090
  ``` 

If you are using VSCode, there is a convenient launch configuration in `.vscode`.

#### Validate code

- run `make lint`
- run `make test`
- run `make swagger-api-server`
