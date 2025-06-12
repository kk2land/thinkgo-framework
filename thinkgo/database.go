package thinkgo

import (
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
	"sync"
	"time"
	"xorm.io/xorm"
)

// DBErrRetry 判断err是否是可以重试的错误
func DBErrRetry(err error) bool {
	//判断是socket超时 or socket关闭
	if ErrIsTimeout(err) || ErrIsBrokenPipe(err) {
		return true
	}
	//判断是不合法的连接
	s := err.Error()
	if s == "invalid connection" {
		return true
	}
	//mysql
	{
		var e *mysql.MySQLError
		if errors.As(err, &e) {
			switch e.Number {
			//1213 = ER_LOCK_DEADLOCK
			//1205 = ER_LOCK_WAIT_TIMEOUT
			//2006 = CR_SERVER_GONE_ERROR
			//2013 = CR_SERVER_LOST
			//1047 = ER_UNKNOWN_COM_ERROR
			//1158–1161 = 网络相关错误（如 ER_NET_READ_ERROR）
			case 1213, 1205, 2002, 2006, 2013, 1047, 1158, 1159, 1160, 1161:
				return true
			}
		}
	}
	//postgresql
	{
		var e *pq.Error
		if errors.As(err, &e) {
			switch e.Code {
			//40001 = serialization_failure
			//40P01 = deadlock_detected
			//55P03 = lock_not_available
			//57014 = query_canceled
			//08000 = connection_exception
			//08001 = sqlclient_unable_to_establish_sqlconnection
			//08003 = connection_does_not_exist
			//08006 = connection_failure
			case "40001", "40P01", "55P03", "57014", "08000", "08001", "08003", "08006":
				return true
			}
		}
	}
	//sqlite3
	{
		var e sqlite3.Error
		if errors.As(err, &e) {
			switch {
			case errors.Is(e.Code, sqlite3.ErrBusy), errors.Is(e.Code, sqlite3.ErrLocked):
				return true
			case errors.Is(e.ExtendedCode, sqlite3.ErrBusySnapshot), errors.Is(e.ExtendedCode, sqlite3.ErrLockedSharedCache):
				return true
			}
		}
	}
	return false
}

func DBErrDuplicate(err error) bool {
	//mysql
	{
		var e *mysql.MySQLError
		if errors.As(err, &e) {
			return e.Number == 1062
		}
	}
	//postgresql
	{
		var e *pq.Error
		if errors.As(err, &e) {
			return e.Code == "23505"
		}
	}
	//sqlite3
	{
		var e sqlite3.Error
		if errors.As(err, &e) {
			return errors.Is(e.ExtendedCode, sqlite3.ErrConstraintUnique)
		}
	}
	return false
}

var dbInstanceMap = NewSyncMap(dbCreate)
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

func dbCreate(name string) (*DBInstance, error) {
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
			engine.SetConnMaxIdleTime(config.ConnMaxIdleTime.Duration())
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
			engineGroup.Master().SetConnMaxIdleTime(config.ConnMaxIdleTime.Duration())
			for _, slave := range engineGroup.Slaves() {
				slave.SetConnMaxIdleTime(config.ConnMaxIdleTime.Duration())
			}
		}
		db.EngineInterface = engineGroup
		db.IsCluster = true
	}
	db.Name = name
	if config.MaxIdleConns != 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.getMaxOpenConns() > 0 {
		db.SetMaxOpenConns(config.getMaxOpenConns())
	}
	if config.ConnMaxLifetime != 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime.Duration())
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
	return dbInstanceMap.LoadOrCreate(name)
}

// DBOrPanic 基于name获取数据库操作对象，否则panic
func DBOrPanic(name string) *DBInstance {
	if d, err := DB(name); err != nil {
		panic(err)
	} else {
		return d
	}
}

// DBCloseAll 关闭全部数据对象
func DBCloseAll() {
	dbInstanceMap.Clear(func(name string, db *DBInstance) {
		var err error
		if db.IsCluster {
			err = (db.EngineInterface.(*xorm.EngineGroup)).Close()
		} else {
			err = (db.EngineInterface.(*xorm.Engine)).Close()
		}
		if err != nil {
			Logger.Errorf("[DBCloseAll]close db[%s] fail - %s", db.Name, err.Error())
		}
	})
}
