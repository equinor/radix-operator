FROM golang:alpine3.7 as builder

RUN apk update && apk add git && apk add -y ca-certificates curl && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN mkdir -p /go/src/github.com/statoil/radix-operator/
WORKDIR /go/src/github.com/statoil/radix-operator/
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only
COPY ./pipeline-runner ./pipeline-runner
COPY ./pkg ./pkg
WORKDIR /go/src/github.com/statoil/radix-operator/pipeline-runner/

ARG date
ARG commitid
ARG branch

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -X main.pipelineCommitid=${commitid} -X main.pipelineBranch=${branch} -X main.pipelineDate=${date}" -a -installsuffix cgo -o ./rootfs/pipeline-runner
RUN adduser -D -g '' radix-pipeline

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/src/github.com/statoil/radix-operator/pipeline-runner/rootfs/pipeline-runner /usr/local/bin/pipeline-runner
USER radix-pipeline
ENTRYPOINT ["/usr/local/bin/pipeline-runner"]