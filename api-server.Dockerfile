# Build stage
FROM --platform=$BUILDPLATFORM docker.io/golang:1.26.2-alpine3.23 AS builder
ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY ./api-server ./api-server
COPY ./job-scheduler ./job-scheduler
COPY ./pkg ./pkg
WORKDIR /src/api-server
RUN go build -ldflags="-s -w" -o /build/radix-api

# Final stage, ref https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md for distroless
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /build/radix-api .
USER 1000
ENTRYPOINT ["/app/radix-api"]
