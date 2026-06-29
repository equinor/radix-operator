#!/bin/sh
# Authorize ANY presented public key by echoing it back as an authorized_keys line.
# sshd invokes this command (AuthorizedKeysCommand) with the connecting client's key as
# "<key-type> <base64-key>", so emitting the same value authorizes exactly that key. This lets the
# operator's auto-generated deploy key authenticate without provisioning authorized_keys in advance.
# Safe only because this server runs in a throwaway, isolated e2e kind cluster.
printf '%s %s\n' "$1" "$2"
