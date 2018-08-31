FROM golang:alpine as builder
RUN apk update && apk add git && apk add ca-certificates
RUN adduser -D -g '' appuser

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY rootfs/pipeline-runner /usr/local/bin/pipeline-runner
USER appuser
ENTRYPOINT ["/usr/local/bin/pipeline-runner"]