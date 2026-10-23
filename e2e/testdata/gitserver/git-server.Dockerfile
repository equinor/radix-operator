# Minimal SSH git server used by the e2e tests as a stand-in for github.com.
# It serves repositories over SSH and accepts any client public key (see any-key.sh),
# so the operator-generated deploy key authenticates without any prior key exchange. The host key
# and repository content are injected at runtime via mounted Secret/ConfigMap.
FROM alpine:3.20

RUN apk add --no-cache openssh-server git \
    && adduser -D -h /srv/git -s /usr/bin/git-shell git \
    && sed -i 's/^git:!/git:*/' /etc/shadow \
    && mkdir -p /srv/git /etc/gitserver \
    && chown -R git:git /srv/git

COPY sshd_config /etc/gitserver/sshd_config
COPY any-key.sh /usr/local/bin/any-key
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/any-key /usr/local/bin/entrypoint.sh

EXPOSE 22
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
