package gpgsql

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ClarkQAQ/gpgsql/release"
)

const (
	CliInit pgCliMethod = iota
	CliStart
	CliStop
	CliRestart
	CliStatus
	CliReload
	CliPromote
	CliLogrotate
	CliKill
)

var (
	pgCliBinary         string = filepath.Join(binaryRootPath, release.PgCliBinary)
	pgCliMethods               = []string{"init", "start", "stop", "restart", "status", "reload", "promote", "logrotate", "kill"}
	defaultPgCliOptions        = &PgCliOptions{
		Wait:    true,
		Timeout: 5,
	}
)

type pgCliMethod uint8

func (p pgCliMethod) String() string {
	if len(pgCliMethods) < int(p) {
		return ""
	}

	return pgCliMethods[p]
}

type PgCliOptions struct {
	Silent    bool     // only print errors, no informational messages
	Timeout   int      // seconds to wait when using wait option
	Wait      bool     // wait until operation completes
	CoreFiles bool     // allow postgres to produce core files (only on start or restart)
	Mode      string   // can be "smart", "fast", or "immediate" (only on stop or restart)
	Options   []string // command line options to pass to postgres (PostgreSQL server executable) or initdb
	Args      []string // command line arguments
}

func (g *GpgsqlRuntime) PgCli(ctx context.Context, method pgCliMethod, opts ...*PgCliOptions) (e error) {
	if len(opts) < 1 || opts[0] == nil {
		opts = append(opts, defaultPgCliOptions)
	}

	opt := opts[0]

	args := []string{
		method.String(),
		"--pgdata", g.data,
	}

	if opt.Silent {
		args = append(args, "--silent")
	}

	if opt.Timeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(opt.Timeout))
	}

	if opt.Wait {
		args = append(args, "--wait")
	} else {
		args = append(args, "--no-wait")
	}

	if opt.CoreFiles {
		args = append(args, "--core-files")
	}

	if opt.Mode != "" {
		args = append(args, "--mode", opt.Mode)
	}

	if len(opt.Options) > 0 {
		args = append(args, "--options", strings.Join(opt.Options, " "))
	}

	if len(opt.Args) > 0 {
		args = append(args, opt.Args...)
	}

	cmd := exec.CommandContext(ctx, pgCliBinary, args...)
	hookWriter := NewHookWriter(g.logger)

	cmd.Stdout = g.logger
	cmd.Stderr = hookWriter
	cmd.Dir = binaryRootPath

	if e := cmd.Run(); e != nil {
		return fmt.Errorf("failed to execute command: %s", e.Error())
	}

	if e := hookWriter.Error(); e != nil {
		return fmt.Errorf("stderr: %s", e.Error())
	}

	return nil
}
