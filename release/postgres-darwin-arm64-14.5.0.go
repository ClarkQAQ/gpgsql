//go:build darwin && arm64

// This file generated by cmd/gen/main.go - DO NOT EDIT

package release

import (
	_ "embed"
)

const (
	Target  = "darwin"
	Arch    = "arm64"
	Version = "14.5.0"
    Sha256  = "459e8a05311ad842fa1ca5043e269daf43398814addd1648d277a88e8d3734bd"

	InitdbBinary = "bin/initdb"
	PgCliBinary  = "bin/pg_ctl"
	PostgresBinary = "bin/postgres"
)

var (
	//go:embed postgres-darwin-arm64-14.5.0.tar.xz
	Archive []byte
)

