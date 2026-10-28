#!/bin/sh
if [ -e /lib/ld-linux-armhf.so.3 ]; then
 runtime=/mnt/us/weread/bin/kindlehf/script-runtime
else
 runtime=/mnt/us/weread/bin/kindlepw2/script-runtime
fi
[ -f "$runtime" ] && [ ! -L "$runtime" ] || exit 1
chmod 755 "$runtime" || exit 1
exec "$runtime" s08dc5aec3c17061db0d1b9cfbcfd09ffefc49e41ea550f41ff35a3db29a65186 "$0" "$@"
