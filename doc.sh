#!/bin/sh

dir=$(cd "$(dirname "$0")"; pwd)
cd $dir

godoc_bin="$GOPATH/bin/godoc"
if [ ! -f "$godoc_bin" ];then
    echo "安装godoc"
    GOFLAGS="" go install golang.org/x/tools/cmd/godoc@latest
fi

echo "http://127.0.0.1:6060/pkg/git.hy545.cc/crypto/thinkgo-framework/thinkgo"
$godoc_bin -http=:6060
