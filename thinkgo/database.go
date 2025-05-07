package thinkgo

import (
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"sync"
	"time"
	"xorm.io/xorm"
)

// DBErrRetry 判断err是否是可以重试的错误
func DBErrRetry(err error) bool {
	if ErrIsTimeout(err) || ErrIsBrokenPipe(err) {
		return true
	}
	s := err.Error()
	if s == "invalid connection" {
		return true
	}
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		switch mysqlErr.Number {
		//1213 = deadlock
		case 2002, 2006, 2013, 1213:
			return true
		}
	}
	return false
}

var dbInstanceMap = NewInstanceMap(dbCreate)
var dbInstanceDefault *DBInstance
var dbInstanceOnce sync.Once

// DBInstance 数据库操作对象
type DBInstance struct {
	xorm.EngineInterface
	Name      string
	IsCluster bool
}

// ExecWithBackoff 带判断错误是可以重试的错误则会进行错误操作，函数f可以panic错误，或者return错误
func (db *DBInstance) ExecWithBackoff(backoff BackoffPolicy, f func(*DBInstance) error) error {
	cb := func() (err error) {
		defer func() {
			if err1 := recover(); err1 != nil {
				err = Recover2Error(err)
			}
		}()
		return f(db)
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

func (db *DBInstance) Close() (err error) {
	dbInstanceMap.Delete(db.Name)
	if db.IsCluster {
		eg := db.EngineInterface.(*xorm.EngineGroup)
		err = eg.Close()
	} else {
		e := db.EngineInterface.(*xorm.Engine)
		err = e.Close()
	}
	if err != nil {
		Logger.Errorf("[DBInstance]close db[%s] fail - %s", db.Name, err.Error())
	}
	return
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

// DBDefault 获取默认的数据库操作对象
func DBDefault() (*DBInstance, error) {
	dbInstanceOnce.Do(func() {
		dbInstanceDefault, _ = DB("default")
	})
	if dbInstanceDefault != nil {
		return dbInstanceDefault, nil
	}
	return nil, errors.New("db[default] not exists")
}

// DBDefaultOrPanic 获取默认的数据库操作对象，获取失败则panic
func DBDefaultOrPanic() *DBInstance {
	if d, err := DBDefault(); err != nil {
		panic(err)
	} else {
		return d
	}
}

// DB 基于name获取数据库操作对象
func DB(name string) (*DBInstance, error) {
	obj, err := dbInstanceMap.LoadOrCreate(name)
	if err != nil {
		return nil, err
	}
	return obj.(*DBInstance), nil
}

// DB 基于name获取数据库操作对象，否则panic
func DBOrPanic(name string) *DBInstance {
	if d, err := DB(name); err != nil {
		panic(err)
	} else {
		return d
	}
}

// DBCloseAll 关闭全部数据对象
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
