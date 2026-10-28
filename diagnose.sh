#!/bin/sh
if [ -e /lib/ld-linux-armhf.so.3 ]; then
 runtime=/mnt/us/weread/bin/kindlehf/script-runtime
else
 runtime=/mnt/us/weread/bin/kindlepw2/script-runtime
fi
[ -f "$runtime" ] && [ ! -L "$runtime" ] || exit 1
chmod 755 "$runtime" || exit 1
exec "$runtime" se16310628c9676ac5f2c825d27f1471d3f531be9baaaec16579294eec6bd8922 "$0" "$@"
