#!/bin/sh
if [ -e /lib/ld-linux-armhf.so.3 ]; then
 runtime=/mnt/us/weread/bin/kindlehf/script-runtime
else
 runtime=/mnt/us/weread/bin/kindlepw2/script-runtime
fi
[ -f "$runtime" ] && [ ! -L "$runtime" ] || exit 1
chmod 755 "$runtime" || exit 1
exec "$runtime" saad64fbd987d7ab10bdd367a166d8d841ed909a499b40e99d6462f4855f4ecbe "$0" "$@"
