FROM golang:alpine3.9 as builder

RUN apk update && apk add git && apk add -y ca-certificates curl dep

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
COPY ./radix-operator ./radix-operator
COPY ./pkg ./pkg
WORKDIR /go/src/github.com/equinor/radix-operator/radix-operator/

# Run tests
RUN CGO_ENABLED=0 GOOS=linux go test ./... ../pkg/...

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o ./rootfs/radix-operator
RUN adduser -D -g '' radix-operator

# Run operator
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/src/github.com/equinor/radix-operator/radix-operator/rootfs/radix-operator /usr/local/bin/radix-operator
USER radix-operator
ENTRYPOINT ["/usr/local/bin/radix-operator"]
