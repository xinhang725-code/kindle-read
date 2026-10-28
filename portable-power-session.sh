#!/bin/sh
if [ -e /lib/ld-linux-armhf.so.3 ]; then
 runtime=/mnt/us/weread/bin/kindlehf/script-runtime
else
 runtime=/mnt/us/weread/bin/kindlepw2/script-runtime
fi
[ -f "$runtime" ] && [ ! -L "$runtime" ] || exit 1
chmod 755 "$runtime" || exit 1
exec "$runtime" s720ac6952e2dabb550d99a14fb3e608b27b8178793df296a4c795a4d332bb8fc "$0" "$@"
