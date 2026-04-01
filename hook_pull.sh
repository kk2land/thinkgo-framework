#!/bin/sh

module=""
echo "$PROJECT_NAME"

function app_build() {
    module="$1"
    src_dir="src"

    tk_module="git.hy545.cc/crypto/thinkgo-framework"
    if [ -z "$module" ];then
        ldflags=""
        main_arr=$(ls $src_dir/main/*.go)
        cmd_arr=$(ls $src_dir/cmd/*.go)
        dst_dir="app/bin"
    else
        ldflags="-X \"${tk_module}/thinkgo.ModuleName=${module}\""
        main_arr=$(ls $src_dir/${module}/main/*.go)
        cmd_arr=$(ls $src_dir/${module}/cmd/*.go)
        dst_dir="app/${module}/bin"
    fi

    rm -fr $dst_dir/*
    for main in $main_arr
    do
        base=`basename ${main} '.go'`
        if [[ "$base" == "x" ]];then
            continue
        fi
        echo "go build -ldflags "${ldflags}" -o ${dst_dir}/${base} $main"
        /usr/local/go/bin/go build -ldflags "${ldflags}" -o ${dst_dir}/${base} $main
        if [ $? -ne 0 ];then
            return 1
        fi
        chmod 755 ${dst_dir}/${base}
    done

    for cmd in $cmd_arr
    do
        base=`basename ${cmd} '.go'`
        if [[ "$base" == "x" ]];then
            continue
        fi
        c=$(cat << EOF
#!/bin/sh
bin_dir=\$(cd "\$(dirname "\$0")"; pwd)
export _TK_Command=$base
exec \$bin_dir/cmd ${base} "\$@"
EOF
)
        printf "$c\n" >$dst_dir/$base
        chmod +x $dst_dir/$base
    done
}

app_build "$module"