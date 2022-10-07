package gpgsql

import (
	"context"
	"errors"
	"fmt"
	"gpgsql/release"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	if strings.TrimSpace(g.data) == "" {
		return errors.New("data directory is empty")
	}

	if strings.TrimSpace(g.username) == "" {
		return errors.New("username is empty")
	}

	args := []string{
		"--pgdata", g.data,
		"--username", g.username,
	}

	if strings.TrimSpace(g.password) != "" {
		f, e := os.CreateTemp(os.TempDir(), "*")
		if e != nil {
			return fmt.Errorf("failed to create temp file: %s", e.Error())
		}

		defer os.Remove(f.Name())

		if _, e := f.WriteString(g.password); e != nil {
			return fmt.Errorf("failed to write password to temp file: %s", e.Error())
		}

		args = append(args, "--pwfile", f.Name())
	}

	if strings.TrimSpace(opt.Encoding) != "" {
		args = append(args, "--encoding", opt.Encoding)
	}

	if opt.NoLocale {
		args = append(args, "--no-locale")
	} else if strings.TrimSpace(opt.Locale) != "" {
		args = append(args, "--locale", opt.Locale)
	}

	if strings.TrimSpace(opt.AuthMethod) != "" {
		args = append(args, "--auth", opt.AuthMethod)
	}

	if opt.DataChecksums {
		args = append(args, "--data-checksums")
	}

	if strings.TrimSpace(opt.TextSearchConfig) != "" {
		args = append(args, "--text-search-config", opt.TextSearchConfig)
	}

	if len(opt.Args) > 0 {
		args = append(args, opt.Args...)
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
