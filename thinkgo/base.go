package thinkgo

import (
	"errors"
	"fmt"
	"github.com/fvbock/endless"
	"github.com/joho/godotenv"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const (
	envKeyRootPath         = "_TK_RootPath"
	envKeyAppStatus        = "_TK_AppStatus"
	envKeyInternalHttpPort = "_TK_Internal_HttpPort"
	envKeyLogConsole       = "_TK_LogConsole"

	CommandNameHttp  = "http"
	CommandNameGrpc  = "grpc"
	CommandNameCheck = "check"
)

var (
	ModuleName  string //指定module名称，编译时指定
	InModule    bool   //是否启动的module
	RootPath    string //项目根路径
	AppPath     string
	ConfigPath  string
	RuntimePath string

	CommandName string
	AppName     string
	AppStatus   string
	AppDebug    bool
	Hostname    string
	Pid         int

	Config *appConfig
	Logger FieldLogger

	opsAlarmGoQueue *GoQueue
	opsAlarmSyslog  *syslog.Writer = nil
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
	CommandName = filepath.Base(os.Args[0])

	envFile := filepath.Join(RootPath, ".env")
	if IsFile(envFile) {
		err = godotenv.Load(envFile)
		if err != nil {
			panic(err)
		}
	}

	envAppStatus := os.Getenv(envKeyAppStatus)
	if len(envAppStatus) > 0 {
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
	if InModule {
		AppName = Config.AppName + "-" + ModuleName
	} else {
		AppName = Config.AppName
	}
	//初始化日志配置
	Logger = iniAppLogger(Config.CmdLogNames[CommandName])
	Logger.Infof("AppStatus = %s, AppDebug = %t", AppStatus, AppDebug)

	opsAlarmGoQueue = NewGoQueue(10, func(obj interface{}) {
		var err error
		if opsAlarmSyslog == nil {
			if opsAlarmSyslog, err = syslog.New(syslog.LOG_ERR|syslog.LOG_LOCAL6, AppName); err != nil {
				Logger.Errorf("opsAlarmSyslog new fail,err=%s", err)
				return
			}
		}
		str := obj.(string)
		if err := opsAlarmSyslog.Err(str); err != nil {
			Logger.Errorf("opsAlarmSyslog write fail,err=%s,str=%s", err, str)
			_ = opsAlarmSyslog.Close()
			opsAlarmSyslog = nil
			return
		}
	})
	opsAlarmGoQueue.Start()

	//todo
	switch CommandName {
	case CommandNameHttp, CommandNameCheck:
		initHttpServer()
	}
	initRedis()
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
	ConfigPath = filepath.Join(AppPath, "config")
	if !IsDir(ConfigPath) {
		return fmt.Errorf("config path not dir - %s", ConfigPath)
	}
	RuntimePath = filepath.Join(AppPath, "runtime")
	if !IsDir(RuntimePath) {
		return fmt.Errorf("runtime path not dir - %s", RuntimePath)
	}
	return nil
}

func OpsAlarm(format string, v ...interface{}) {
	OpsAlarmWithGroup(Config.OpsAlarm, format, v...)
}

func OpsAlarmWithGroup(group string, format string, v ...interface{}) {
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
	opsAlarmGoQueue.Send(buf.String())
}

func WritePidFile() error {
	return WriteFile(filepath.Join(RuntimePath, "pid", AppName+".pid"), []byte(strconv.Itoa(Pid)))
}

var startHooks = struct {
	hooks []func()
	mu    sync.Mutex
	done  bool
}{}
var shutdownHooks = struct {
	hooks []func(wait *sync.WaitGroup)
	mu    sync.Mutex
	done  bool
}{}

func AddStartHook(h func()) {
	startHooks.mu.Lock()
	defer startHooks.mu.Unlock()
	if startHooks.done {
		panic(errors.New("server已经启动，不能再AddStartHook"))
	}
	startHooks.hooks = append(startHooks.hooks, h)
}

func CallStartHooks() {
	startHooks.mu.Lock()
	defer startHooks.mu.Unlock()
	for _, h := range startHooks.hooks {
		h()
	}
	startHooks.done = true
}

func AddShutdownHook(h func(wait *sync.WaitGroup)) {
	shutdownHooks.mu.Lock()
	defer shutdownHooks.mu.Unlock()
	if shutdownHooks.done {
		panic(errors.New("server已经启动，不能再AddShutdownHook"))
	}
	shutdownHooks.hooks = append(shutdownHooks.hooks, h)
}

func CallShutdownHooks() {
	shutdownHooks.mu.Lock()
	defer shutdownHooks.mu.Unlock()
	wait := sync.WaitGroup{}
	for _, h := range shutdownHooks.hooks {
		h(&wait)
	}
	wait.Wait()
	shutdownHooks.done = true
	Logger.Infof("CallShutdownHooks finished")
}

type RegisterSignalHook func(prePost int, sig os.Signal, f func()) error

func StartHttpServer(
	port int,
	handler http.Handler,
	options func(server *http.Server, registerSignalHook RegisterSignalHook),
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
			httpPort = server.EndlessListener.Addr().(*net.TCPAddr).Port
		} else {
			httpPort = port
		}
		_ = os.Setenv(envKeyInternalHttpPort, strconv.Itoa(httpPort))
		Logger.Infof("[StartHttpServer]开始启动httpserver - %d", httpPort)
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

func ListenShutdownSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
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
