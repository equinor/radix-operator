FROM golang:alpine3.10 as builder

ENV GO111MODULE=on

RUN apk update && \
    apk add git ca-certificates curl && \
    apk add --no-cache gcc musl-dev && \
    go get -u golang.org/x/lint/golint github.com/frapposelli/wwhrd@v0.2.4

WORKDIR /go/src/github.com/equinor/radix-operator/

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

# Check dependency licenses using https://github.com/frapposelli/wwhrd
COPY .wwhrd.yml ./
RUN wwhrd -q check

# Copy project code
COPY ./radix-operator ./radix-operator
COPY ./pkg ./pkg

# Run tests
RUN golint `go list ./... | grep -v "pkg/client"` && \
    go vet `go list ./... | grep -v "pkg/client"` && \
    CGO_ENABLED=0 GOOS=linux go test `go list ./... | grep -v "pkg/client"`

# Build
WORKDIR /go/src/github.com/equinor/radix-operator/radix-operator/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o ./rootfs/radix-operator
RUN addgroup -S -g 1000 radix-operator
RUN adduser -S -u 1000 -G radix-operator radix-operator

# Run operator
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/src/github.com/equinor/radix-operator/radix-operator/rootfs/radix-operator /usr/local/bin/radix-operator
USER radix-operator
ENTRYPOINT ["/usr/local/bin/radix-operator"]
