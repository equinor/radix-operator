#!/bin/sh
set -e

# Install the host key provided via the mounted secret. Keeping it out of the image means the
# known_hosts entry can be generated together with the key by the test harness.
install -m 600 -o root -g root /etc/gitserver-keys/ssh_host_ed25519_key /etc/gitserver/ssh_host_ed25519_key

# Serve whatever repositories the test harness packaged into the mounted archive. The server is
# generic: it has no knowledge of the repository layout or its contents, it just extracts the
# bare repositories into the served directory (idempotent).
export HOME=/srv/git
if [ -z "$(ls -A /srv/git 2>/dev/null)" ]; then
  tar -xzf /etc/gitserver-repo/repo.tar.gz -C /srv/git
  chown -R git:git /srv/git
fi

exec /usr/sbin/sshd -D -e -f /etc/gitserver/sshd_config
