FROM golang:1.17.6-alpine3.15 as builder

RUN apk update && \
    apk add ca-certificates curl git  && \
    apk add --no-cache gcc musl-dev && \
    go get -u golang.org/x/lint/golint github.com/frapposelli/wwhrd

WORKDIR /go/src/github.com/equinor/radix-operator/

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

# Check dependency licenses using https://github.com/frapposelli/wwhrd
COPY .wwhrd.yml ./
RUN wwhrd -q check

# Copy project code
COPY ./pipeline-runner ./pipeline-runner
COPY ./pkg ./pkg

# Run tests
RUN golint `go list ./... | grep -v "pkg/client"` && \
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