package gpgsql

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gpgsql/release"
	"io"
	"net"

	_ "github.com/lib/pq"
)

func ReleaseTarget() string {
	return release.Target
}

func ReleaseArch() string {
	return release.Arch
}

func ReleaseVersion() string {
	return release.Version
}

func ReleaseSha256() string {
	return release.Sha256
}

func ReleaseArchive() []byte {
	return release.Archive
}

type HookWriter struct {
	w    io.Writer
	hook func(p []byte)
	buf  *bytes.Buffer
}

func NewHookWriter(w io.Writer) *HookWriter {
	return &HookWriter{
		w:    w,
		hook: nil,
		buf:  bytes.NewBuffer(nil),
	}
}

func (w *HookWriter) Write(p []byte) (n int, e error) {
	if w.hook != nil {
		w.hook(p)
	}

	if w.w != nil {
		w.w.Write(p)
	}

	return w.buf.Write(p)
}

func (w *HookWriter) Error() error {
	if w.buf.Len() < 1 {
		return nil
	}

	return errors.New(w.buf.String())
}

func (w *HookWriter) Hook(f func(p []byte)) {
	w.hook = f
}

// getFreePort returns an available TCP port that is ready to use.
func (g *GpgsqlRuntime) getFreePort() (uint16, error) {
	addr, e := net.ResolveTCPAddr("tcp",
		(&net.TCPAddr{IP: g.host, Port: 0}).String())
	if e != nil {
		return 0, e
	}

	l, e := net.ListenTCP("tcp", addr)
	if e != nil {
		return 0, e
	}
	defer l.Close()

	return uint16(l.Addr().(*net.TCPAddr).Port), nil
}

func (g *GpgsqlRuntime) CheckConnection(ctx context.Context) error {
	db, e := sql.Open("postgres", g.DSN(g.username))
	if e != nil {
		return fmt.Errorf("connection to %s failed: %s", g.DSN(g.username), e.Error())
	}

	defer db.Close()

	if _, e = db.QueryContext(ctx, "SELECT 1"); e != nil {
		return fmt.Errorf("connection to %s failed: %s", g.DSN(g.username), e.Error())
	}

	return nil
}

func (g *GpgsqlRuntime) DSN(dbname string) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		g.host.String(), g.port, g.username, g.password, dbname)
}

func (g *GpgsqlRuntime) DB(dbname string) (*sql.DB, error) {
	return sql.Open("postgres", g.DSN(dbname))
}
