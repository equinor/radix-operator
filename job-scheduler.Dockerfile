FROM --platform=$BUILDPLATFORM docker.io/golang:1.26.0-alpine3.23 AS builder
ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src

COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./job-scheduler ./job-scheduler
COPY ./pkg ./pkg
WORKDIR /src/job-scheduler
RUN go build -ldflags="-s -w" -o /build/radix-job-scheduler

# Final stage, ref https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md for distroless
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /build/radix-job-scheduler .
USER 1000
ENTRYPOINT ["/app/radix-job-scheduler"]