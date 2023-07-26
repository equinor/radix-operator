FROM golang:1.18-alpine3.17 as base

RUN apk update && \
    apk add ca-certificates curl git  && \
    apk add --no-cache gcc musl-dev && \
    go install honnef.co/go/tools/cmd/staticcheck@2023.1.3

WORKDIR /go/src/github.com/equinor/radix-operator/

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy project code
COPY ./pipeline-runner ./pipeline-runner
COPY ./pkg ./pkg

FROM base as run-staticcheck
RUN staticcheck `go list ./... | grep -v "pkg/client"` && touch /staticcheck.done

FROM base as tester
# Run tests
RUN go vet `go list ./... | grep -v "pkg/client"` && \
    CGO_ENABLED=0 GOOS=linux go test `go list ./... | grep -v "pkg/client"` && \
    touch /tests.done

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
# This will make sure staticcheck and tests are run before the final stage is built
COPY --from=run-staticcheck /staticcheck.done /staticcheck.done
COPY --from=tester /tests.done /tests.done
USER radix-pipeline
ENTRYPOINT ["/usr/local/bin/pipeline-runner"]