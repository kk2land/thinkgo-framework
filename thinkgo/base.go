package thinkgo

import (
	"errors"
	"flag"
	"fmt"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/fvbock/endless"
	"github.com/joho/godotenv"
)

const (
	envKeyRootPath         = "_TK_RootPath"          // go run时，设置使用
	envKeyAppStatus        = "_TK_AppStatus"         // 环境配置加载
	envKeyAppDebug         = "_TK_AppDebug"          // '1'则开启debug模式
	envKeyInternalHttpPort = "_TK_Internal_HttpPort" // http服务内部使用，对于随机的端口，reload后保持端口不变
	envKeyLogConsole       = "_TK_LogConsole"        // '1'则直接将日志输出到屏幕上
	envKeyCommand          = "_TK_Command"           // 可以指定新的CommandName
)

var (
	ModuleName  string //指定module名称，编译时指定
	InModule    bool   //是否在module下启动
	RootPath    string //项目根路径
	AppPath     string //应用路径 = app or app/{module}
	configPath  string //配置路径 = app/config or app/{module}/config
	RuntimePath string //运行时文件路径 = app/runtime or app/{module}/runtime

	CommandName string //当前启动的二进制名
	AppName     string //从配置中读取的应用名 config.AppName
	AppStatus   string //环境变量或.env文件读取的_TK_AppStatus，用来判断app运行环境
	AppDebug    bool   //是否是调试模式允许，Logger.Debug(f)会打印出来，从配置文件读取
	Hostname    string //当前主机的hostname
	Pid         int    //当前的进程pid

	Config *appConfig  //当前应用的配置对象
	Logger FieldLogger //当前应用的logger对象

	opsAlarmQueueRef atomic.Pointer[opsAlarmQueue]

	commands = make(map[string]func())
)

func init() {
	var err error
	//初始化模块名
	InModule = len(ModuleName) > 0

	//初始化路径信息
	envRootPath := os.Getenv(envKeyRootPath) //读取rootPath
	var rootPath string
	if len(envRootPath) > 0 {
		rootPath = envRootPath
	} else {
		if rootPath, err = filepath.Abs(os.Args[0]); err != nil {
			panic(err)
		}
		// cmd -> bin -> module -> app -> rootPath
		rootPath = filepath.Dir(filepath.Dir(filepath.Dir(rootPath)))
		if InModule {
			rootPath = filepath.Dir(rootPath)
		}
	}
	err = initPathVars(rootPath)
	if err != nil {
		panic(err)
	}

	//初始化各种变量
	if CommandName = os.Getenv(envKeyCommand); CommandName == "" {
		CommandName = filepath.Base(os.Args[0])
	}

	envFile := filepath.Join(RootPath, ".env")
	if IsFile(envFile) {
		err = godotenv.Load(envFile)
		if err != nil {
			panic(err)
		}
	}

	if envAppStatus := os.Getenv(envKeyAppStatus); len(envAppStatus) > 0 {
		AppStatus = envAppStatus
		AppDebug = !strings.HasPrefix(AppStatus, "prod")
	}

	if Hostname, err = os.Hostname(); err != nil {
		panic(err)
	}
	Pid = os.Getpid()

	//初始化配置
	Config, err = initAppConfig()
	if err != nil {
		panic(err)
	}
	if Config.AppDebug {
		AppDebug = Config.AppDebug
	}
	if envAppDebug := os.Getenv(envKeyAppDebug); len(envAppDebug) > 0 {
		AppDebug = envAppDebug == "1"
	}

	AppName = Config.AppName
	//初始化日志配置
	Logger = iniAppLogger(Config.CmdLogNames[CommandName])
	Logger.Infof("AppStatus = %s, _TK_AppDebug = %t", AppStatus, AppDebug)

	AddStartHook(func() {
		queue := &opsAlarmQueue{}
		queue.GoQueue = NewGoQueue(10, func(obj interface{}) {
			queue.directSend(obj.(string))
		})
		queue.Start()
		opsAlarmQueueRef.Store(queue)
	})
	AddShutdownHook(func(wait *sync.WaitGroup) {
		if queue := opsAlarmQueueRef.Load(); queue != nil {
			queue.CloseAndWait(wait)
		}
	})

	initRedis()
}

type opsAlarmQueue struct {
	*GoQueue
	opsAlarmSyslog *syslog.Writer
}

func (m *opsAlarmQueue) directSend(s string) {
	var err error
	if m.opsAlarmSyslog == nil {
		if m.opsAlarmSyslog, err = syslog.New(syslog.LOG_ERR|syslog.LOG_LOCAL6, AppName); err != nil {
			Logger.Errorf("opsAlarmSyslog new fail,err=%s", err)
			return
		}
	}
	if err = m.opsAlarmSyslog.Err(s); err != nil {
		Logger.Errorf("opsAlarmSyslog write fail,err=%s,str=%s", err, s)
		_ = m.opsAlarmSyslog.Close()
		m.opsAlarmSyslog = nil
		return
	}
}

func initPathVars(dir string) error {
	if !IsDir(dir) {
		return fmt.Errorf("apppath not dir - %s", dir)
	}
	var err error
	if RootPath, err = filepath.Abs(dir); err != nil {
		return err
	}
	if InModule {
		AppPath = filepath.Join(RootPath, "app", ModuleName)
	} else {
		AppPath = filepath.Join(RootPath, "app")
	}
	if !IsDir(AppPath) {
		return fmt.Errorf("app path not dir - %s", AppPath)
	}
	configPath = filepath.Join(AppPath, "config")
	if !IsDir(configPath) {
		return fmt.Errorf("config path not dir - %s", configPath)
	}
	RuntimePath = filepath.Join(AppPath, "runtime")
	if !IsDir(RuntimePath) {
		return fmt.Errorf("runtime path not dir - %s", RuntimePath)
	}
	return nil
}

// CommandRegister 命令入口注册
func CommandRegister(name string, f func()) interface{} {
	commands[name] = f
	return nil
}

// CommandRun 命令执行
func CommandRun() {
	cmd := os.Args[1]
	if f, ok := commands[cmd]; !ok {
		panic(fmt.Errorf("command(%s)不存在-%s", cmd))
	} else {
		os.Args = os.Args[1:]
		flag.CommandLine.Init(cmd, flag.PanicOnError)
		SetProcessTitle(AppPath + "/bin/" + cmd)
		f()
	}
}

// OpsAlarm 进行tg告警，要依赖运维部署环境
func OpsAlarm(format string, v ...interface{}) {
	OpsAlarmWithGroup(Config.OpsAlarm, false, format, v...)
}

func OpsAlarmDirect(format string, v ...interface{}) {
	OpsAlarmWithGroup(Config.OpsAlarm, true, format, v...)
}

// OpsAlarmWithGroup 进行tg告警，可以指定告警组
func OpsAlarmWithGroup(group string, direct bool, format string, v ...interface{}) {
	Logger.Errorf(format, v...)
	buf := BytesBuffer1024.Get()
	defer BytesBuffer1024.Put(buf)

	buf.WriteString("OPS_ALARM")
	if len(group) > 0 {
		buf.WriteByte('[')
		buf.WriteString(group)
		buf.WriteByte(']')
	}
	buf.WriteByte(' ')
	buf.WriteString(fmt.Sprintf(format, v...))
	if buf.Len() > 1024 {
		buf.Truncate(1024)
	}

	var queue *opsAlarmQueue
	if direct {
		goto direct
	}
	if queue = opsAlarmQueueRef.Load(); queue == nil {
		goto direct
	}
	queue.TrySend(buf.String())
	return

direct:
	//如果是命令行运行，则直接调用发送错误
	queue = &opsAlarmQueue{}
	queue.directSend(buf.String())
	if queue.opsAlarmSyslog != nil {
		_ = queue.opsAlarmSyslog.Close()
	}
}

// WritePidFile 写当前进程的pid文件
func WritePidFile() error {
	return WriteFile(filepath.Join(RuntimePath, "pid", AppName+".pid"), []byte(strconv.Itoa(Pid)))
}

type shutdownHook struct {
	id int32
	f  func(wait *sync.WaitGroup)
}

var startHooks = struct {
	hooks []func()
	mu    sync.Mutex
	done  bool
}{}
var shutdownHooks = struct {
	hooks []shutdownHook
	mu    sync.Mutex
	done  bool
}{}
var shutdownHooksIdCounter atomic.Int32

// AddStartHook 添加http-server等服务启动时的回调
func AddStartHook(h func()) {
	startHooks.mu.Lock()
	defer startHooks.mu.Unlock()
	if startHooks.done {
		h()
		return
	}
	startHooks.hooks = append(startHooks.hooks, h)
}

// CallStartHooks 触发http-server等服务启动时的回调，以下使用情景自动有效
//
//	StartHttpServer
//	ListenShutdownSignals
//	HttpStartServer
func CallStartHooks() {
	startHooks.mu.Lock()
	defer startHooks.mu.Unlock()
	for _, h := range startHooks.hooks {
		h()
	}
	startHooks.done = true
	startHooks.hooks = nil
}

// AddShutdownHook 添加服务停止时的回调
func AddShutdownHook(h func(wait *sync.WaitGroup)) int32 {
	shutdownHooks.mu.Lock()
	defer shutdownHooks.mu.Unlock()
	if shutdownHooks.done {
		panic(errors.New("server已经停止，不能再AddShutdownHook"))
	}
	id := shutdownHooksIdCounter.Add(1)
	shutdownHooks.hooks = append(shutdownHooks.hooks, shutdownHook{id, h})
	return id
}

// RemoveShutdownHook 移除服务停止回调
func RemoveShutdownHook(id int32) {
	shutdownHooks.mu.Lock()
	defer shutdownHooks.mu.Unlock()
	if !shutdownHooks.done {
		for i, hook := range shutdownHooks.hooks {
			if hook.id == id {
				shutdownHooks.hooks = append(shutdownHooks.hooks[:i], shutdownHooks.hooks[i+1:]...)
				break
			}
		}
	}
}

// CallShutdownHooks 触发服务停止时的回调，以下使用情景自动有效
//
//	StartHttpServer
//	ListenShutdownSignals
//	HttpStartServer
func CallShutdownHooks() {
	shutdownHooks.mu.Lock()
	defer shutdownHooks.mu.Unlock()
	wait := sync.WaitGroup{}
	for _, hook := range shutdownHooks.hooks {
		hook.f(&wait)
	}
	wait.Wait()
	shutdownHooks.done = true
	shutdownHooks.hooks = nil
	Logger.Infof("CallShutdownHooks finished")
}

// HttpServerRegisterSignalHook http-server注册信号处理回调
type HttpServerRegisterSignalHook func(prePost int, sig os.Signal, f func()) error

// StartHttpServer 不使用gin框架直接启动http-server(kill -HUP {pid}，支持平滑重启)
func StartHttpServer(
	port int,
	handler http.Handler,
	options func(server *http.Server, registerSignalHook HttpServerRegisterSignalHook),
) {
	defer func() {
		DBCloseAll()
		RedisCloseAll()
	}()
	if port < 0 {
		port = 0
	}
	addr := fmt.Sprintf(":%d", port)
	server := endless.NewServer(addr, handler)
	//配置server
	options(&server.Server, server.RegisterSignalHook)
	//对于随机端口，获取port
	server.BeforeBegin = func(add string) {
		if port == 0 {
			httpPort.Store(int32(server.EndlessListener.Addr().(*net.TCPAddr).Port))
		} else {
			httpPort.Store(int32(port))
		}
		_ = os.Setenv(envKeyInternalHttpPort, strconv.Itoa(HttpPort()))
		Logger.Infof("[StartHttpServer]开始启动httpserver - %d", HttpPort())
		if err := WritePidFile(); err != nil {
			Logger.Errorf("[StartHttpServer]写入pidfile失败 - %s", err)
			panic(err)
		}
		CallStartHooks()
	}
	_ = server.RegisterSignalHook(endless.POST_SIGNAL, syscall.SIGINT, CallShutdownHooks)
	_ = server.RegisterSignalHook(endless.POST_SIGNAL, syscall.SIGTERM, CallShutdownHooks)

	if err := server.ListenAndServe(); err != nil {
		if !strings.Contains(err.Error(), "use of closed network connection") {
			Logger.Errorf("[StartHttpServer]启动httpserver失败 - %s", err)
			panic(err)
		} else {
			Logger.Info("[StartHttpServer]关闭httpserver")
		}
	}
}

// ListenShutdownSignals 不启动http-server，直接启动一个运行服务，监听信号退出
func ListenShutdownSignals() {
	defer func() {
		DBCloseAll()
		RedisCloseAll()
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	Logger.Infof("[ListenShutdownSignals]开始启动server")
	if err := WritePidFile(); err != nil {
		Logger.Errorf("[ListenShutdownSignals]写入pidfile失败 - %s", err)
		panic(err)
	}
	CallStartHooks()

loop:
	for {
		select {
		case <-ch:
			Logger.Infof("服务开始关闭-%s", AppName)
			CallShutdownHooks()
			Logger.Infof("服务已关闭-%s", AppName)
			break loop
		}
	}
}
