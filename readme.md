# thinkgo-framework - 自研go的web框架

## 部署

添加框架依赖
```bash
go env -w GOPRIVATE="git.hy545.cc/crypto/*"
go get git.hy545.cc/crypto/thinkgo-framework
```
1. 将`app_init.sh`和`hook_pull.sh`复制到项目根目录下.

2. 执行`app_init.sh {module}`脚本，生成项目的目录结构，同时参数`{module}`为空，则表示不需要module层级

3. 如果有module，则修改`hook_pull.sh`，给函数`app_build`传module参数

## 使用

### 配置

- 配置格式参考`app.toml`
- 支持读取项目根目录下`.env`文件
  - 如果配置了`_TK_AppStatus`则会额外读取`app/config/app_{_TK_AppStatus}.toml`
- module的配置文件读取顺序
  - app/config/app.toml
  - app/config/app_{_TK_AppStatus}.toml
  - app/config/{module}/config/app.toml
  - app/config/{module}/config/app_{_TK_AppStatus}.toml

### 数据库使用

数据库直接使用的[xorm](https://xorm.io/zh/docs/)，举个例子：
```go

// 默认的表名、字段名使用驼峰转成小写下划线分隔的方案

type MyStudent struct {
    Id         int64 `xorm:"pk autoincr"` //指定自增主键
    Name       string
    CreateTime int64 `xorm:"ct"` //指定字段名
}

func (m *MyStudent) TableName() string {
	// 指定表名
	return "student"
}

db := thinkgo.DBDefaultOrPanic()

// 插入
studentA := &MyStudent{
	Name: "A",
    CreateTime: time.Now().Unix()
}
affected, err := db.Insert(studentA)

// 读取多条
students := make([]*MyStudent, 0)
err := db.Find(&students)

//读取单条
student := new(MyStudent)
has, err := db.Where("id = ?", 1).Get(student)

```