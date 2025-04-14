package thinkgo

import (
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"sync"
	"time"
	"xorm.io/xorm"
)

// DBDefaultOrPanic() or DBOrPanic("") 获取xorm实例

func DBErrRetry(err error) bool {
	if ErrIsTimeout(err) || ErrIsBrokenPipe(err) {
		return true
	}
	s := err.Error()
	if s == "invalid connection" {
		return true
	}
	return false
}

var dbInstanceMap = NewInstanceMap(dbCreate)
var dbInstanceDefault *DBInstance
var dbInstanceOnce sync.Once

type DBInstance struct {
	xorm.EngineInterface
	Name      string
	IsCluster bool
}

func (db *DBInstance) Exec1(backoff BackoffPolicy, f func() error) error {
	cb := func() (err error) {
		defer func() {
			if err1 := recover(); err1 != nil {
				err = Recover2Error(err)
			}
		}()
		return f()
	}
	var err error
	for backoff.Next() {
		if err = cb(); err == nil {
			return nil
		} else if !DBErrRetry(err) {
			return err
		} else if backoff.End() {
			return err
		}
		time.Sleep(backoff.Get())
	}
	return nil
}

func dbCreate(name string) (interface{}, error) {
	var config dbConfig
	var ok bool
	if config, ok = Config.DB[name]; !ok {
		return nil, fmt.Errorf("config db[%s] not exists", name)
	}
	//初始化xorm引擎
	var db DBInstance
	var err error
	if len(config.DriverSourceName) > 0 {
		var engine *xorm.Engine
		engine, err = xorm.NewEngine(config.DriverName, config.DriverSourceName)
		if err != nil {
			return nil, err
		}
		if config.ConnMaxLifetime != 0 {
			engine.SetConnMaxIdleTime(config.ConnMaxIdleTime.ToDuration())
		}
		db.EngineInterface = engine
		db.IsCluster = false
	} else {
		var engineGroup *xorm.EngineGroup
		engineGroup, err = xorm.NewEngineGroup(config.DriverName, config.DriverSourceNames)
		if err != nil {
			return nil, err
		}
		if config.ConnMaxIdleTime != 0 {
			engineGroup.Master().SetConnMaxIdleTime(config.ConnMaxIdleTime.ToDuration())
			for _, slave := range engineGroup.Slaves() {
				slave.SetConnMaxIdleTime(config.ConnMaxIdleTime.ToDuration())
			}
		}
		db.EngineInterface = engineGroup
		db.IsCluster = true
	}
	db.Name = name
	db.SetMaxIdleConns(0)
	if config.getMaxOpenConns() > 0 {
		db.SetMaxOpenConns(config.getMaxOpenConns())
	}
	if config.ConnMaxLifetime != 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime.ToDuration())
	}

	db.SetLogger(&dbLogger{FieldLogger: Logger})
	return &db, nil
}

func DBDefault() (*DBInstance, error) {
	dbInstanceOnce.Do(func() {
		dbInstanceDefault, _ = DB("default")
	})
	if dbInstanceDefault != nil {
		return dbInstanceDefault, nil
	}
	return nil, errors.New("db[default] not exists")
}

func DBDefaultOrPanic() *DBInstance {
	if d, err := DBDefault(); err != nil {
		panic(err)
	} else {
		return d
	}
}

func DB(name string) (*DBInstance, error) {
	obj, err := dbInstanceMap.LoadOrCreate(name)
	if err != nil {
		return nil, err
	}
	return obj.(*DBInstance), nil
}

func DBOrPanic(name string) *DBInstance {
	if d, err := DB(name); err != nil {
		panic(err)
	} else {
		return d
	}
}

func DBCloseAll() {
	dbInstanceMap.Clear(func(name string, inst interface{}) {
		var err error
		if db, ok := inst.(*DBInstance); ok {
			if db.IsCluster {
				eg := db.EngineInterface.(*xorm.EngineGroup)
				err = eg.Close()
			} else {
				e := db.EngineInterface.(*xorm.Engine)
				err = e.Close()
			}
			if err != nil {
				Logger.Errorf("[DBCloseAll]close db[%s] fail - %s", db.Name, err.Error())
			}
		}
	})
}
