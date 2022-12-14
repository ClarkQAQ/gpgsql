//go:build linux && arm

// This file generated by cmd/gen/main.go - DO NOT EDIT

package release

import (
	_ "embed"
)

const (
	Target  = "linux"
	Arch    = "arm"
	Version = "14.5.0"
	Sha256  = "0e81951a4f56a12a4492acc9c0e9c809d895d0e771ccde30b9d557ebf297dc19"

	InitdbBinary   = "bin/initdb"
	PgCliBinary    = "bin/pg_ctl"
	PostgresBinary = "bin/postgres"
)

var (
	//go:embed postgres-linux-arm-14.5.0.tar.xz
	Archive []byte
)
