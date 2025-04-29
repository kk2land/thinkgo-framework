#!/bins/h

dir=$(cd "$(dirname "$0")"; pwd)
cd $dir

module="$1"

app_path="app/$module"

if [ -z "$module" ];then
    mkdir -p src/cmd
else
    mkdir -p src/$module/cmd
fi

mkdir -p "$app_path"
mkdir "$app_path/config"
mkdir "$app_path/bin"
mkdir "$app_path/runtime"
mkdir "$app_path"/runtime/{log,pid}

touch $app_path/bin/.gitkeep

echo ".gitignore内容"
echo "/.idea"
echo "app/bin/*"
echo "app/runtime/log/*"
echo "app/runtime/pid/*"
echo "app/*/bin/*"
echo "app/*/runtime/log/*"
echo "app/*/runtime/pid/*"