package thinkgo

import (
	"errors"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"regexp"
)

// 框架配置类

type appConfig struct {
	AppName     string            `toml:"app_name"`
	AppDebug    bool              `toml:"app_debug"`
	OpsAlarm    string            `toml:"ops_alarm"`
	CmdLogNames map[string]string `toml:"cmd_logNames"`
	Http        HttpConfig
	Redis       map[string]redisConfig
	DB          map[string]dbConfig `toml:"database"`
	Custom      JMap
}

type HttpConfig struct {
	Port           int
	WithGrpc       bool
	ReadTimeout    ConversionDuration
	WriteTimeout   ConversionDuration
	IdleTimeout    ConversionDuration
	MaxHeaderBytes int
}

type redisOptionConfig struct {
	Timeout         ConversionDuration
	PoolSize        int
	MaxRetries      int
	MinRetryBackoff ConversionDuration
	MaxRetryBackoff ConversionDuration
}

type redisConfig struct {
	Addr     string
	Password string
	DB       int `toml:"db"`
	Prefix   string
	redisOptionConfig
	CmdOptions map[string]redisOptionConfig `toml:"cmd_options"`
}

func (c redisConfig) getOptions() redisOptionConfig {
	if optConfig, ok := c.CmdOptions[CommandName]; ok {
		return optConfig
	}
	return c.redisOptionConfig
}

type dbOptionConfig struct {
	MaxOpenConns int
	MaxIdleConns int
}

type dbConfig struct {
	DriverName        string
	DriverSourceName  string
	DriverSourceNames []string
	ConnMaxLifetime   ConversionDuration
	ConnMaxIdleTime   ConversionDuration
	dbOptionConfig
	CmdOptions map[string]dbOptionConfig `toml:"cmd_options"`
}

func (c dbConfig) getOptions() dbOptionConfig {
	if optConfig, ok := c.CmdOptions[CommandName]; ok {
		return optConfig
	}
	return c.dbOptionConfig
}

func initAppConfig() (*appConfig, error) {
	c := new(appConfig)

	var files []string
	files = append(files, filepath.Join(RootPath, "app", "config", "app.toml"))
	if len(AppStatus) > 0 {
		files = append(files, filepath.Join(RootPath, "app", "config", "app_"+AppStatus+".toml"))
	}
	if InModule {
		files = append(files, filepath.Join(ConfigPath, "app.toml"))
		if len(AppStatus) > 0 {
			files = append(files, filepath.Join(ConfigPath, "app_"+AppStatus+".toml"))
		}
	}

	re := regexp.MustCompile(`\{\{\s*([\w-.]*)\s*}}`)
	for _, file := range files {
		println("load config file - " + file)
		if IsFile(file) {
			buf, err := ReadFile(file)
			if err != nil {
				return nil, err
			}
			str := string(buf)
			str = re.ReplaceAllStringFunc(str, func(m string) string {
				subMatches := re.FindStringSubmatch(m)
				if len(subMatches) > 1 {
					key := subMatches[1]
					var val string
					switch key {
					case "ModuleName":
						val = ModuleName
					case "AppPath":
						val = AppPath
					case "AppStatus":
						val = AppStatus
					case "CommandName":
						val = CommandName
					case "Hostname":
						val = Hostname
					default:
						val = os.Getenv(key)
					}
					return val
				}
				return m
			})
			if _, err := toml.Decode(str, c); err != nil {
				return nil, err
			}
		}
	}
	if len(c.AppName) == 0 {
		return nil, errors.New("config[AppName]不能为空")
	}
	return c, nil
}
