FROM alpine:3.6

RUN adduser -D radix-operator
USER radix-operator
COPY rootfs/radix-operator /usr/local/bin/radix-operator
CMD ["/usr/local/bin/radix-operator"]