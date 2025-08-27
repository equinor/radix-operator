FROM --platform=$BUILDPLATFORM docker.io/golang:1.24.6-alpine3.21 AS builder
ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src

COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./operator ./operator
COPY ./pkg ./pkg
WORKDIR /src/operator
RUN go build -ldflags="-s -w" -o /build/radix-operator

# Final stage, ref https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md for distroless
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /build/radix-operator .
USER 1000
ENTRYPOINT ["/app/radix-operator"]