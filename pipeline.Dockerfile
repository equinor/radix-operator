FROM golang:alpine3.9 as builder

RUN apk update && apk add git && apk add ca-certificates curl dep

RUN mkdir -p /go/src/github.com/equinor/radix-operator/
WORKDIR /go/src/github.com/equinor/radix-operator/

# Install project dependencies
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only

# Check dependency licenses using https://github.com/frapposelli/wwhrd
COPY .wwhrd.yml ./
RUN go get -u github.com/frapposelli/wwhrd
RUN wwhrd -q check

# Copy project code
COPY ./pipeline-runner ./pipeline-runner
COPY ./pkg ./pkg
WORKDIR /go/src/github.com/equinor/radix-operator/pipeline-runner/

# Run tests
RUN CGO_ENABLED=0 GOOS=linux go test ./... ../pkg/...

# Build
ARG date
ARG commitid
ARG branch
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -X main.pipelineCommitid=${commitid} -X main.pipelineBranch=${branch} -X main.pipelineDate=${date}" -a -installsuffix cgo -o ./rootfs/pipeline-runner
RUN adduser -D -g '' radix-pipeline

# Run operator
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/src/github.com/equinor/radix-operator/pipeline-runner/rootfs/pipeline-runner /usr/local/bin/pipeline-runner
USER radix-pipeline
ENTRYPOINT ["/usr/local/bin/pipeline-runner"]