package main

import (
	"context"
	"fmt"
	"gpgsql"
	"time"
	"utilware/logger"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

func main() {
	logger.Info("target: %s, arch: %s, version: %s, sha256: %s",
		gpgsql.ReleaseTarget(), gpgsql.ReleaseArch(), gpgsql.ReleaseVersion(), gpgsql.ReleaseSha256())

	g, e := gpgsql.New()
	if e != nil {
		logger.Fatal("new gpgsql error: %s", e.Error())
	}

	g.Logger(nil)

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

	{
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if e := g.Stop(ctx); e != nil {
			logger.Fatal("stop failed: %s", e.Error())
		}
	}

	{
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if e := g.Start(ctx, &gpgsql.PostgreSqlOptions{
			Wait:    3 * time.Second,
			Timeout: 5 * time.Second,
		}); e != nil {
			logger.Fatal("postgresql start failed: %s", e.Error())
		}

		logger.Info("postgresql is running")
		logger.Debug("postgresql listening on %s", g.ListenAddr())
	}

	defer func() {
		if e := g.Stop(context.Background()); e != nil {
			logger.Fatal("postgresql stop failed: %s", e.Error())
		}

		logger.Info("postgresql stop success")
	}()

	{
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		if e := PgTest(ctx, g); e != nil {
			logger.Fatal("postgresql pg test failed: %s", e.Error())
		}
	}

	{
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		if e := SqlTest(ctx, g); e != nil {
			logger.Fatal("postgresql sql test failed: %s", e.Error())
		}
	}
}

func SqlTest(ctx context.Context, g *gpgsql.GpgsqlRuntime) error {
	db, e := g.DB(g.GetUsername())
	if e != nil {
		return e
	}

	row := db.QueryRow(`
	SELECT 
		"user_model"."id", 
		"user_model"."name"
	FROM "user_models" AS "user_model" 
	WHERE (id = 1)
	`)

	user := &UserModel{}

	if e := row.Scan(&user.Id, &user.Name); e != nil {
		return e
	}

	logger.Info("sql user: %s", user)

	return nil
}

type UserModel struct {
	Id     int64
	Name   string
	Emails []string
}

func (u UserModel) String() string {
	return fmt.Sprintf("User<%d %s %v>", u.Id, u.Name, u.Emails)
}

func PgTest(ctx context.Context, g *gpgsql.GpgsqlRuntime) error {
	db := pg.Connect(&pg.Options{
		Network:  "tcp",
		Addr:     g.ListenAddr(),
		User:     g.GetUsername(),
		Password: g.GetPassword(),
		Database: g.GetUsername(),
	})

	defer db.Close()

	db.AddQueryHook(&DebugHook{Verbose: true, EmptyLine: true})

	if e := db.Model(&UserModel{}).CreateTable(&orm.CreateTableOptions{
		IfNotExists: true,
	}); e != nil {
		return e
	}

	if _, e := db.Model(&UserModel{
		Name:   "admin",
		Emails: []string{"admin@localhost", "admin@localhost"},
	}).Where("id = ?", 1).SelectOrInsert(); e != nil {
		return e
	}

	if e := db.Model(&UserModel{}).Where("id > ?", 0).ForEach(func(u *UserModel) error {
		logger.Info("pg user: %s", u)
		return nil
	}); e != nil {
		return e
	}

	return nil
}

type DebugHook struct {
	Verbose   bool
	EmptyLine bool
}

func (h *DebugHook) BeforeQuery(ctx context.Context, evt *pg.QueryEvent) (context.Context, error) {
	return ctx, nil
}

func (h *DebugHook) AfterQuery(ctx context.Context, evt *pg.QueryEvent) error {
	q, e := evt.FormattedQuery()
	if e != nil {
		return e
	}

	if evt.Err != nil {
		logger.Printf("[pgdebug] %s\r\n[%s] %s", time.Since(evt.StartTime), q, evt.Err)
	} else if h.Verbose {
		if evt.Result != nil {
			logger.Printf("[pgdebug] %s aff: %d ret: %d\r\n%s",
				time.Since(evt.StartTime), evt.Result.RowsAffected(),
				evt.Result.RowsReturned(), q)
		} else {
			logger.Printf("[pgdebug] %s\r\n%s", time.Since(evt.StartTime), q)
		}
	}

	if h.EmptyLine {
		fmt.Println()
	}

	return nil
}
