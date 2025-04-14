#!/bin/sh

dir=$(cd "$(dirname "$0")"; pwd)
cd $dir

tk_module="git.hy545.cc/crypto/thinkgo-framework"

module="$1"

if [ -z "$module" ];then
    ldflags=""
    cmd_arr=$(ls src/cmd/*.go)
    dst="app/bin"
else
    ldflags="-X \"${tk_module}/thinkgo.ModuleName=${module}\""
    cmd_arr=$(ls src/${module}/cmd/*.go)
    dst="app/${module}/bin"
fi

rm -fr $dst/*
for cmd in $cmd_arr
do
    base=`basename ${cmd} '.go'`
    if [[ "$base" == "x" ]];then
        continue
    fi
    echo "go build -ldflags "${ldflags}" -o ${dst}/${base} $cmd"
    /usr/local/go/bin/go build -ldflags "${ldflags}" -o ${dst}/${base} $cmd
    if [ $? -ne 0 ];then
        exit 1
    fi
done