<p align="center">
  <h3 align="center">Gpgsql</h3>
  <p align="center">
    一个嵌入式的PostgreSQL数据库, 以及命令行绑定
    <br />
    An embedded PostgreSQL database, and command line binding
    <br />
  </p>
</p>


### 关于Gpgsql

Golang 内嵌数据库一直是一大难题, Sqlite 虽然现在终于有了纯 Golang [sqlite](https://gitlab.com/cznic/sqlite), 但是它的性能相对其他, 于是我就想着能不能用 PostgreSQL 来实现一个嵌入式的数据库, 直接把性能最大化, 于是就有了这个项目. 

在社区里面, 我看到了一个叫 [embedded-postgres](https://github.com/fergusstrange/embedded-postgres) 的项目, 但是它的二进制文件需要实时下载, 而且命令行参数也不够灵活, 所以我就自己写了一个, 也算是对它的一个补充.

目前这个项目的二进制会自动根据编译时的 target 来嵌入对应的 PostgreSQL 二进制文件, 也就是说, 你可以在不同的平台上编译, 但是在运行时不需要安装 PostgreSQL, 也不需要下载二进制文件, 也不需要配置环境变量, 也不需要配置 PATH, 也不需要配置任何东西, 只需要在你的代码里面引入这个包, 然后就可以直接使用了.


### 现有问题:

1. 目前只有 Linux 和 Windows 是测试过的, Mac 由于没有 Mac 机器, 所以暂时没有测试过, 但是理论上是可以的, 因为 Linux 是可以运行的, 所以理论上是可以的, 但是我没有测试过, 如果你有 Mac 机器, 可以帮忙测试一下, 如果有问题, 可以提 issue, 我会尽快修复的.
2. 在 Windows 上面如果用管理员权限运行的就不能使用 `xxx.Daemon` 的方式来启动, 管理员权限会导致 `xxx.Daemon` 无法正常工作, 就只能用 `xxx.Start` 的方式来启动, `pg_cli` 好像会自动处理, 不过在 Posix 系统上面就必须用非管理员运行了,请不要做 root 敢死队...
3. 由于上游没有提供 Unix (Openbsd/Freebsd) 的二进制包, 所以目前没办法支持这些平台, 如果有人有兴趣, 可以自己编译二进制包, 然后提 PR, 我会合并的. 

### 示例 (Example)：[Example](https://github.com/ClarkQAQ/gpgsql/tree/master/example)

### 演示 (Demo)：

```go
g, e := gpgsql.New()
if e != nil {
	logger.Fatal("new gpgsql error: %s", e.Error())
}

if e := g.Username("postgres").
	Password("postgres").
	Data("data/"); e != nil {
	logger.Fatal("add data failed: %s", e.Error())
}

if t, e := g.IsEmptyData(); e != nil {
	logger.Fatal("is empty data failed: %s", e.Error())
} else if t {
	if e := g.Initdb(context.Background(), &gpgsql.InitdbOptions{
		Encoding:   "UTF8",
		Locale:     "en_US.UTF-8",
		AuthMethod: "password",
	}); e != nil {
		logger.Fatal("initdb failed: %s", e.Error())
	}
}

if e := g.Start(context.Background(), &gpgsql.PostgreSqlOptions{
	Wait:    3 * time.Second,
	Timeout: 5 * time.Second,
}); e != nil {
	logger.Fatal("postgresql start failed: %s", e.Error())
}

```

### TODO:

1. 添加 ACM 自动机来匹配输出, 以便更好的返回错误信息.
2. 添加更多的测试用例.
3. 进一步优化接口, 使其更加易用.
4. `IsEmptyData` 这个方法的实现有点问题, 在某些情况下应该会误判, 以后再改吧.

### 参考项目:
    
[灵感来源](https://github.com/fergusstrange/embedded-postgres)
[二进制包](https://github.com/zonkyio/embedded-postgres-binaries)

### 版权说明

该项目签署了 MIT 授权许可，随意参与以及使用!