package gpgsql

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ClarkQAQ/gpgsql/release"
)

var (
	initdbBinary = filepath.Join(binaryRootPath, release.InitdbBinary)

	defaultInitdbOptions = &InitdbOptions{
		Encoding:      "UTF8",
		AuthMethod:    "password",
		DataChecksums: true,
	}
)

type InitdbOptions struct {
	Encoding         string
	Locale           string
	NoLocale         bool
	AuthMethod       string
	TextSearchConfig string
	DataChecksums    bool
	Args             []string
}

func (g *GpgsqlRuntime) Initdb(ctx context.Context, opts ...*InitdbOptions) (e error) {
	if len(opts) < 1 || opts[0] == nil {
		opts = append(opts, defaultInitdbOptions)
	}

	opt := opts[0]

	args, e := initdbArgs(g, opt)
	if e != nil {
		return e
	}

	cmd := exec.CommandContext(ctx, initdbBinary, args...)

	hookWriter := NewHookWriter(g.logger)
	cmd.Stdout = g.logger
	cmd.Stderr = hookWriter
	cmd.Dir = binaryRootPath

	if e := cmd.Run(); e != nil {
		if e := hookWriter.Error(); e != nil {
			return fmt.Errorf("stderr: %s", e.Error())
		}

		if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
			return errors.New("exit status is not successful")
		}

		return fmt.Errorf("failed to execute command: %s", e.Error())
	}

	return nil
}

func initdbArgs(g *GpgsqlRuntime, opt *InitdbOptions) ([]string, error) {
	args := []string{
		"--pgdata", g.data,
		"--username", g.username,
	}

	switch {
	case strings.TrimSpace(g.data) == "":
		return nil, errors.New("data directory is empty")
	case strings.TrimSpace(g.username) == "":
		return nil, errors.New("username is empty")
	case strings.TrimSpace(g.password) != "":
		f, e := os.CreateTemp(os.TempDir(), "*")
		if e != nil {
			return nil, fmt.Errorf("failed to create temp file: %s", e.Error())
		}

		defer os.Remove(f.Name())

		if _, e := f.WriteString(g.password); e != nil {
			return nil, fmt.Errorf("failed to write password to temp file: %s", e.Error())
		}

		args = append(args, "--pwfile", f.Name())
		fallthrough
	case strings.TrimSpace(opt.Encoding) != "":
		args = append(args, "--encoding", opt.Encoding)
		fallthrough
	case opt.NoLocale:
		args = append(args, "--no-locale")
		fallthrough
	case strings.TrimSpace(opt.Locale) != "":
		args = append(args, "--locale", opt.Locale)
		fallthrough
	case strings.TrimSpace(opt.AuthMethod) != "":
		args = append(args, "--auth", opt.AuthMethod)
		fallthrough
	case opt.DataChecksums:
		args = append(args, "--data-checksums")
		fallthrough
	case strings.TrimSpace(opt.TextSearchConfig) != "":
		args = append(args, "--text-search-config", opt.TextSearchConfig)
	}

	args = append(args, opt.Args...)

	return args, nil
}
