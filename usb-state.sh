#!/bin/sh
if [ -e /lib/ld-linux-armhf.so.3 ]; then
 runtime=/mnt/us/weread/bin/kindlehf/script-runtime
else
 runtime=/mnt/us/weread/bin/kindlepw2/script-runtime
fi
[ -f "$runtime" ] && [ ! -L "$runtime" ] || exit 1
chmod 755 "$runtime" || exit 1
exec "$runtime" s0f143f8c550512a87f89b6506b176e615299aa073c6a283775714c9c079dab93 "$0" "$@"
