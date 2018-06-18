FROM alpine:3.7

RUN adduser -D radix-operator
RUN apk add --no-cache ca-certificates curl && update-ca-certificates
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin/kubectl
USER radix-operator
COPY rootfs/rx /usr/local/bin/rx
CMD ["/usr/local/bin/rx"]