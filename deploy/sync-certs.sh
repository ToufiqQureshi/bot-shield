#!/bin/sh
# Copy Certbot's current lineage into a read-only container mount. The Go
# process runs as UID/GID 65532 and cannot read Certbot's root-only key.
set -eu

lineage=${1:?usage: sync-certs.sh <certbot-lineage> [destination]}
target=${2:-/etc/hakaishield/tls}
[ -f "$lineage/fullchain.pem" ] && [ -f "$lineage/privkey.pem" ] || {
    echo "certificate lineage is incomplete: $lineage" >&2
    exit 1
}

if [ "$target" = /etc/hakaishield/tls ]; then
    install -d -m 0755 -o 0 -g 0 /etc/hakaishield
fi
install -d -m 0750 -o 0 -g 65532 "$target"
cert_tmp=$(mktemp "$target/.fullchain.XXXXXX")
key_tmp=$(mktemp "$target/.privkey.XXXXXX")
trap 'rm -f "$cert_tmp" "$key_tmp"' EXIT
install -m 0644 -o 0 -g 65532 "$lineage/fullchain.pem" "$cert_tmp"
install -m 0640 -o 0 -g 65532 "$lineage/privkey.pem" "$key_tmp"
mv -f "$cert_tmp" "$target/fullchain.pem"
mv -f "$key_tmp" "$target/privkey.pem"
