#!/bin/sh

bin_dir=$(cd "$(dirname "$0")"; pwd)
root_dir=$(dirname ${bin_dir})

whoami=`whoami`
if [ "$whoami" != "www" ];then
    echo "必须以www用户运行"
    exit 1
fi

op="$1"

app_name="..."
pid_file="${root_dir}/runtime/pid/${app_name}.pid"

function start()
{
    echo "开始启动${app_name}进程"
    if [[ -f "$pid_file" ]]; then
        pid=`cat ${pid_file}`
        if [[ -n "$pid" ]];then
            kill -s 0 ${pid} 2>/dev/null
            if [[ $? -eq 0 ]];then
                echo "${app_name}进程(${pid})尚在"
                return 0
            fi
        fi
    fi
    nohup ${root_dir}/bin/metamask >/dev/null 2>&1 &
}

function stop()
{
    echo "开始关闭${app_name}进程"
    if [[ ! -f "$pid_file" ]]; then
        echo "${app_name}进程的pid_file不存在 - ${pid_file}"
        return 1
    fi
    pid=`cat ${pid_file}`
    if [[ -z "$pid" ]];then
        echo "${app_name}进程pid_file空 - ${pid_file}"
        return 1
    fi

    kill -s INT ${pid}
    while [[ 1 ]]
    do
        kill -s 0 ${pid} 2>/dev/null
        if [[ $? -ne 0 ]];then
            echo "${app_name}进程(${pid})完成关闭"
            return 0
        else
            echo "${app_name}进程(${pid})正在关闭..."
            sleep 2
        fi
    done
}

function reload()
{
    echo "开始重载${app_name}进程"
    if [[ ! -f "$pid_file" ]]; then
        echo "${app_name}进程的pid_file不存在 - ${pid_file}"
        return 1
    fi
    pid=`cat ${pid_file}`
    if [[ -z "$pid" ]];then
        echo "${app_name}进程pid_file空 - ${pid_file}"
        return 1
    fi

    kill -s 1 ${pid}
    if [ $? -ne 0 ];then
        echo "${app_name}进程，无法reload"
        return 0
    fi
    while [[ 1 ]]
    do
        pid1=`cat ${pid_file} 2>/dev/null`
        if [[ -z "$pid1" ]] || [[ "$pid1" == "$pid" ]];then
            echo "${app_name}进程(${pid})重载中..."
            sleep 2
        else
            echo "${app_name}进程(${pid1})完成重载"
            return 0
        fi
    done
}

case "$op" in
    "start")
        start
        ;;

    "stop")
        stop
        ;;

    "reload")
        reload
        ;;

    "restart")
        stop
        start
        ;;

    *)
        echo "usage: ${0} start|stop|reload|restart"
        ;;
esac