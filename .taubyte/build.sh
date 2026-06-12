#!/bin/bash

export PATH="/usr/local/go/bin:/usr/local/tinygo/bin:/usr/bin:/bin:/sbin:${PATH:-}"
export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"
export GOSUMDB="${GOSUMDB:-sum.golang.org}"

rm -rf .git .git.mv lib out 2>/dev/null || true

. /utils/wasm.sh

target="${FILENAME:-empty.go}"
if [ "$target" = "." ] || [ -z "$target" ]; then
  target="empty.go"
fi
build "${target}"
ret=$?
echo -n "$ret" > /out/ret-code
exit $ret
