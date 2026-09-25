#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
chmod 0755 "$fixture"
mkdir -p "$fixture/lineage" "$fixture/output"
printf 'certificate-v1\n' > "$fixture/lineage/fullchain.pem"
printf 'private-key-v1\n' > "$fixture/lineage/privkey.pem"

sh "$root/deploy/sync-certs.sh" "$fixture/lineage" "$fixture/output"
cmp "$fixture/lineage/fullchain.pem" "$fixture/output/fullchain.pem"
cmp "$fixture/lineage/privkey.pem" "$fixture/output/privkey.pem"
[ "$(stat -c %a "$fixture/output")" = 750 ]
[ "$(stat -c %a "$fixture/output/privkey.pem")" = 640 ]
[ "$(stat -c %g "$fixture/output/privkey.pem")" = 65532 ]
addgroup -g 65532 tls
adduser -D -u 65532 -G tls tls
su tls -s /bin/sh -c "test -r '$fixture/output/privkey.pem'"
su nobody -s /bin/sh -c "test ! -r '$fixture/output/privkey.pem'"

printf 'certificate-v2\n' > "$fixture/lineage/fullchain.pem"
printf 'private-key-v2\n' > "$fixture/lineage/privkey.pem"
sh "$root/deploy/sync-certs.sh" "$fixture/lineage" "$fixture/output"
cmp "$fixture/lineage/fullchain.pem" "$fixture/output/fullchain.pem"
cmp "$fixture/lineage/privkey.pem" "$fixture/output/privkey.pem"
echo 'certificate sync and renewal permissions passed'
