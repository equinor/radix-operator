FROM golang:1.18.5-alpine3.16 as builder

RUN apk update && \
    apk add ca-certificates curl git  && \
    apk add --no-cache gcc musl-dev && \
    go install honnef.co/go/tools/cmd/staticcheck@v0.3.3

WORKDIR /go/src/github.com/equinor/radix-operator/

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy project code
COPY ./pipeline-runner ./pipeline-runner
COPY ./pkg ./pkg

# Run tests
RUN staticcheck `go list ./... | grep -v "pkg/client"` && \
    go vet `go list ./... | grep -v "pkg/client"` && \
    CGO_ENABLED=0 GOOS=linux go test `go list ./... | grep -v "pkg/client"`

# Build
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