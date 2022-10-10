package gpgsql

import (
	"context"
	"fmt"
	"gpgsql/release"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	postgresBinary string = filepath.Join(binaryRootPath, release.PostgresBinary)

	defaultPostgreSqlOptions = &PostgreSqlOptions{
		Wait:    3 * time.Second,
		Timeout: 5 * time.Second,
	}
)

type GpgsqlRuntime struct {
	host     net.IP // host address
	port     uint16 // host port
	username string // username
	password string // password
	data     string // data directory
	logger   io.Writer
}

type PostgreSqlOptions struct {
	Nbuffers             uint64            // number of shared buffers
	DebugLevel           uint              // debugging level
	DMY                  bool              // use European date input format (DMY)
	FsyncOff             bool              // turn fsync off
	EnableTcpConnections bool              // enable TCP/IP connections
	UnixSocket           string            // path to Unix domain socket
	SSL                  bool              // enable SSL connections
	MaxConnection        uint              // maximum number of connections
	WorkMem              uint64            // memory for query execution
	Parma                map[string]string // set run-time parameter
	Args                 []string          // additional arguments
	Wait                 time.Duration     // wait for server to start
	Timeout              time.Duration     // timeout for check connections
}

func New(forceDecompressBinary ...bool) (*GpgsqlRuntime, error) {
	if len(forceDecompressBinary) < 1 {
		forceDecompressBinary = append(forceDecompressBinary, false)
	}

	if e := DecompressBinary(forceDecompressBinary[0]); e != nil {
		return nil, fmt.Errorf("failed to decompress binary: %s", e.Error())
	}

	return &GpgsqlRuntime{
		host:     net.IP{127, 0, 0, 1},
		port:     0,
		username: "postgres",
		password: "",
		logger:   os.Stdout,
	}, nil
}

func (g *GpgsqlRuntime) Host(host net.IP) *GpgsqlRuntime {
	g.host = host
	return g
}

func (g *GpgsqlRuntime) Port(port uint16) *GpgsqlRuntime {
	g.port = port
	return g
}

func (g *GpgsqlRuntime) Username(username string) *GpgsqlRuntime {
	g.username = username
	return g
}

func (g *GpgsqlRuntime) Password(password string) *GpgsqlRuntime {
	g.password = password
	return g
}

func (g *GpgsqlRuntime) Data(data string) (e error) {
	if !filepath.IsAbs(data) {
		if f, _ := os.Stat(data); f == nil {
			if e = os.MkdirAll(data, os.ModePerm); e != nil {
				return e
			}
		}

		if data, e = filepath.Abs(data); e != nil {
			return e
		}
	}

	f, e := os.Stat(data)
	if e != nil {
		return e
	}

	if f.Mode().Perm()|PostgresqlDataPerm != 0 {
		if e := os.Chmod(data, PostgresqlDataPerm); e != nil {
			return e
		}
	}

	g.data = data
	return nil
}

func (g *GpgsqlRuntime) Logger(logger io.Writer) *GpgsqlRuntime {
	g.logger = logger
	return g
}

func (g *GpgsqlRuntime) DaemonArgs(opt *PostgreSqlOptions) (args []string, e error) {
	args = append(args, "-D", g.data)

	if g.host != nil {
		args = append(args, "-h", g.host.String())
	}

	if g.port < 1 {
		if g.port, e = g.getFreePort(); e != nil {
			return nil, fmt.Errorf("failed to get free port: %s", e.Error())
		}
	}

	args = append(args, "-p", fmt.Sprintf("%d", g.port))

	if opt.Nbuffers > 0 {
		args = append(args, "-B", fmt.Sprintf("%d", opt.Nbuffers))
	}

	if opt.DebugLevel > 0 {
		args = append(args, "-d", fmt.Sprintf("%d", opt.DebugLevel))
	}

	if opt.DMY {
		args = append(args, "-e")
	}

	if opt.FsyncOff {
		args = append(args, "-F")
	}

	if opt.EnableTcpConnections {
		args = append(args, "-i")
	}

	if strings.TrimSpace(opt.UnixSocket) != "" {
		args = append(args, "-k", opt.UnixSocket)
	}

	if opt.SSL {
		args = append(args, "-l")
	}

	if opt.MaxConnection > 0 {
		args = append(args, "-N", fmt.Sprintf("%d", opt.MaxConnection))
	}

	if opt.WorkMem > 0 {
		args = append(args, "-S", fmt.Sprintf("%d", opt.WorkMem))
	}

	if opt.Parma != nil && len(opt.Parma) > 0 {
		for k, v := range opt.Parma {
			args = append(args, "-c", fmt.Sprintf("%s=%s", k, v))
		}
	}

	if opt.Args != nil && len(opt.Args) > 0 {
		args = append(args, opt.Args...)
	}

	return args, nil
}

func (g *GpgsqlRuntime) Daemon(ctx context.Context, opts ...*PostgreSqlOptions) (kill func() error, e error) {
	if len(opts) < 1 || opts[0] == nil {
		opts = append(opts, defaultPostgreSqlOptions)
	}

	opt := opts[0]

	args, e := g.DaemonArgs(opt)
	if e != nil {
		return nil, e
	}

	cmd := exec.CommandContext(ctx, postgresBinary, args...)

	cmd.Stdout = g.logger
	cmd.Stderr = g.logger
	cmd.Dir = binaryRootPath

	if e := cmd.Start(); e != nil {
		return nil, e
	}

	if opt.Wait > 0 {
		time.Sleep(opt.Wait)
	}

	if opt.Timeout < 1 {
		opt.Timeout = defaultPostgreSqlOptions.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), opt.Timeout)
	defer cancel()

	if e := g.CheckConnection(ctx); e != nil {
		return nil, e
	}

	return func() error {
		return cmd.Process.Kill()
	}, nil
}

func (g *GpgsqlRuntime) ListenAddr() string {
	return fmt.Sprintf("%s:%d", g.host.String(), g.port)
}

func (g *GpgsqlRuntime) Start(ctx context.Context, opts ...*PostgreSqlOptions) error {
	if len(opts) < 1 || opts[0] == nil {
		opts = append(opts, defaultPostgreSqlOptions)
	}

	opt := opts[0]

	args, e := g.DaemonArgs(opt)
	if e != nil {
		return e
	}

	if e := g.PgCli(ctx, CliStart, &PgCliOptions{
		Wait:    true,
		Timeout: int(opt.Wait.Seconds()),
		Options: args,
	}); e != nil {
		return e
	}

	if opt.Timeout < 1 {
		opt.Timeout = defaultPostgreSqlOptions.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), opt.Timeout)
	defer cancel()

	if e := g.CheckConnection(ctx); e != nil {
		return fmt.Errorf("failed to check connection: %s", e.Error())
	}

	return nil
}

func (g *GpgsqlRuntime) Stop(ctx context.Context) error {
	if f, _ := os.Stat(filepath.Join(g.data, "postmaster.pid")); f != nil {
		if e := g.PgCli(ctx, CliStop); e != nil {
			return fmt.Errorf("failed to stop postgres: %s", e.Error())
		}
	}

	return nil
}
