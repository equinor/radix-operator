FROM --platform=$BUILDPLATFORM docker.io/golang:1.24.4-alpine3.21 AS builder
# allow go build tool to parse .git repo and read build info
RUN apk add git

ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src

COPY ./go.mod ./go.sum ./
RUN go mod download
COPY . .
WORKDIR /src
RUN go build -ldflags="-s -w" -o /build/webhook ./webhook

# Final stage, ref https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md for distroless
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /build/webhook .
USER 1000
ENTRYPOINT ["/app/webhook"]