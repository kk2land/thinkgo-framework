#!/bin/sh

dir=$(cd "$(dirname "$0")"; pwd)

cmd="$1"
if [[ -z "$cmd" ]];then
    cmd="x"
fi
shift 1

_TK_RootPath="${dir}" go run src/cmd/x.go "${@}"