FROM --platform=$BUILDPLATFORM docker.io/golang:1.27.2-alpine3.24 AS builder
ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN go build -ldflags="-s -w" -o /build/fake-blob-csi .

FROM gcr.io/distroless/static
COPY --from=builder /build/fake-blob-csi /fake-blob-csi
ENTRYPOINT ["/fake-blob-csi"]
