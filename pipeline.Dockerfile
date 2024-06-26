FROM golang:1.22-alpine3.20 as base

RUN apk update && \
    apk add ca-certificates curl git  && \
    apk add --no-cache gcc musl-dev

WORKDIR /go/src/github.com/equinor/radix-operator/

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy project code
COPY ./pipeline-runner ./pipeline-runner
COPY ./pkg ./pkg

# Build
FROM base as builder
WORKDIR /go/src/github.com/equinor/radix-operator/pipeline-runner/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o ./rootfs/pipeline-runner
RUN adduser -D -g '' radix-pipeline

# Run operator
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/src/github.com/equinor/radix-operator/pipeline-runner/rootfs/pipeline-runner /usr/local/bin/pipeline-runner

USER radix-pipeline
ENTRYPOINT ["/usr/local/bin/pipeline-runner"]
