FROM --platform=$BUILDPLATFORM docker.io/golang:1.24-alpine3.22 AS builder

ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY ./velero-plugin ./velero-plugin
COPY ./pkg ./pkg

WORKDIR /src/velero-plugin
RUN go build -ldflags="-s -w" -o /build/radix-velero-plugin

# Final stage
FROM docker.io/alpine:3
COPY --from=builder /build/radix-velero-plugin /plugins/
USER 65534
ENTRYPOINT ["/bin/sh", "-c", "cp /plugins/* /target/."]

