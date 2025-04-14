#!/bins/h

dir=$(cd "$(dirname "$0")"; pwd)
cd $dir

module="$1"

app_path="app/$module"

mkdir -p "$app_path"
mkdir "$app_path/config"
mkdir "$app_path/bin"
mkdir "$app_path/runtime"
mkdir "$app_path"/runtime/{log,pid}