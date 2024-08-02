FROM --platform=$BUILDPLATFORM docker.io/golang:1.22.5-alpine3.20 AS builder
ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src

COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./pipeline-runner ./pipeline-runner
COPY ./pkg ./pkg
WORKDIR /src/pipeline-runner
RUN go build -ldflags="-s -w" -o /build/pipeline-runner

# Final stage, ref https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md for distroless
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /build/pipeline-runner .
USER 1000
ENTRYPOINT ["/app/pipeline-runner"]