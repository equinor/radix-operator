# Radix Job Scheduler
The job scheduler server for application jobs

## Usage
Request from application container URLs
* `POST` `http://<job-name>:8080/api/v1/jobs` - start a new job
* `GET` `http://<job-name>:8080/api/v1/jobs` - get a job list
* `GET` `http://<job-name>:8080/api/v1/jobs/<job-name>` - get a job status
* `DELETE` `http://<job-name>:8080/api/v1/jobs/<job-name>` - delete a job
* `POST` `http://<job-name>:8080/api/v1/jobs/<batch-name>/stop` - stop a batch
* `POST` `http://<job-name>:8080/api/v1/batches` - start a new batch
* `GET` `http://<job-name>:8080/api/v1/batches` - get a batch list
* `GET` `http://<job-name>:8080/api/v1/batches/<batch-name>` - get a batch status
* `DELETE` `http://<job-name>:8080/api/v1/batches/<batch-name>` - delete a batch
* `POST` `http://<job-name>:8080/api/v1/batches/<batch-name>/stop` - stop a batch
* `POST` `http://<job-name>:8080/api/v1/batches/<batch-name>/jobs/<job-name>/stop` - stop a batch job

## Developing

You need Go installed. Run `make bootstrap` to install required tools.

#### Update version
We follow the [semantic version](https://semver.org/) as recommended by [go](https://blog.golang.org/publishing-go-modules).
`radix-job-scheduler` has three places to set version
* `apiVersionRoute` in `router/server.go` and `BasePath`in `docs/docs.go` - API version, used in API's URL
* `Version` in `docs/docs.go` - indicates changes in radix-job-scheduler logic - to see (e.g in swagger), that the version in the environment corresponds with what you wanted

  Run following command to update version in `swagger.json`
    ```
    make swagger
    ``` 

* If generated file `swagger.json` is changed (methods or structures) - copy it to the [public site](https://github.com/equinor/radix-public-site/tree/main/public-site/docs/src/guides/configure-jobs)

### Custom configuration

By default `Info` and `Error` messages are logged. This can be configured via environment variable `LOG_LEVEL` (pods need to be restarted after changes)
* `LOG_LEVEL=ERROR` - log only `Error` messages
* `LOG_LEVEL=INFO` or not set - log `Info` and `Error` messages
* `LOG_LEVEL=WARNING` or not set - log `Info`, `Warning` and `Error` messages
* `LOG_LEVEL=DEBUG` - log `Debug`, `Warning`, `Info` and `Error` messages

By default `swagger UI` is not available. This can be configured via environment variable `USE_SWAGGER`
* `USE_SWAGGER=true` - allows to use swagger UI with URL `<api-endpoint>/swaggerui`

* `USE_PROFILER`
  * `false` or `not set` - do not use profiler
  * `true` - use [pprof](https://golang.org/pkg/net/http/pprof/) profiler, running on `http://localhost:7070/debug/pprof`. Use web-UI to profile, when started service:

  Prerequisite is an installed util `graphviz`
  * Linux: `sudo apt-get install graphviz`
  * Mac: `brew install graphviz`
  
  Run following command to start service with profiler
  ```
  go tool pprof -http=:6070 http://localhost:7070/debug/pprof/heap
  ```
  Compare two states:
  ```bash
  curl -s http://localhost:7070/debug/pprof/heap > ~/tmp/base.heap
  #perform some activity, then grab data again, comparing with the base
  go tool pprof -http=:6070 -base ~/tmp/base.heap http://localhost:7070/debug/pprof/heap
  ```
