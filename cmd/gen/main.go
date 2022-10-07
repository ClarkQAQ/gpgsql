package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"utilware/dep/fasttemplate"

	_ "embed"

	"utilware/logger"

	"utilware/request"
)

const (
	// artifactIdTemplate
	// 1. system target: one of darwin, freebsd, linux, and so on.
	// 2. architecture: one of 386, amd64, arm, and so on.
	artifactIdTemplate = "embedded-postgres-binaries-%[1]s-%[2]s"

	// embedded postgres base url
	// embed artifactIdTemplate
	repositoryBaseURL = "https://repo1.maven.org/maven2/io/zonky/test/postgres/" + artifactIdTemplate

	// embedded postgres version/metadata url
	// embed artifactIdTemplate
	repositoryMetadataURL = repositoryBaseURL + "/maven-metadata.xml"

	// embedded postgres binaries url
	// embed artifactIdTemplate
	// 3. postgres release version
	repositoryBinaryURL = repositoryBaseURL + "/%[3]s/" + artifactIdTemplate + "-%[3]s.jar"

	// release directory
	releaseDirName = "release"

	// release file name
	// 1. system target: one of darwin, freebsd, linux, and so on.
	// 2. architecture: one of 386, amd64, arm, and so on.
	// 3. postgres release version
	// 4. extension name: tar.xz, go
	releaseFileName = "postgres-%[1]s-%[2]s-%[3]s.%[4]s"

	// release info file name
	// releaseInfoName = "release.json"

)

var (
	// system architecture target alias
	archAlias = map[string]string{
		"arm64": "arm64v8",
		"arm":   "arm32v7",
		"386":   "i386",
	}

	// supported system targets and architectures
	supportedTargets = map[string][]string{
		"darwin":  {"amd64", "arm64"},
		"linux":   {"amd64", "386", "arm64", "arm"},
		"windows": {"amd64", "386"},
	}

	targetPath = map[string]map[string]string{
		"darwin": {
			"initdb":   "bin/initdb",
			"pg_ctl":   "bin/pg_ctl",
			"postgres": "bin/postgres",
		},
		"linux": {
			"initdb":   "bin/initdb",
			"pg_ctl":   "bin/pg_ctl",
			"postgres": "bin/postgres",
		},
		"windows": {
			"initdb":   "bin/initdb.exe",
			"pg_ctl":   "bin/pg_ctl.exe",
			"postgres": "bin/postgres.exe",
		},
	}

	proxyAddr = "" // i'm in china, so i need a proxy...

	//go:embed postgres.tmpl
	postgresTmpl string
)

type MavenMetadata struct {
	XMLName    xml.Name `xml:"metadata"`
	Text       string   `xml:",chardata"`
	ArtifactId string   `xml:"artifactId"`
	Versioning struct {
		Text    string `xml:",chardata"`
		Release string `xml:"release"`
	} `xml:"versioning"`
}

type ReleaseInfo struct {
	Time     string               `json:"time"`
	Archives []ReleaseArchiveInfo `json:"archives"`
}

type ReleaseArchiveInfo struct {
	Target  string `json:"target"`
	Arch    string `json:"arch"`
	Version string `json:"version"`
	Sum     string `json:"sum"`
}

func main() {
	flag.StringVar(&proxyAddr, "proxy", "", "proxy address")
	flag.Parse()

	postgresTemplate, e := fasttemplate.NewTemplate(postgresTmpl, "{{", "}}")
	if e != nil {
		logger.Fatal("new template failed: %s", e.Error())
	}

	if f, _ := os.Stat(releaseDirName); f != nil && f.IsDir() {
		if e := os.RemoveAll(releaseDirName); e != nil {
			logger.Fatal("remove release directory failed: %s", e.Error())
		}
	}

	if e := os.MkdirAll(releaseDirName, os.ModePerm); e != nil {
		logger.Fatal("create release directory failed: %s", e.Error())
	}

	for target, archs := range supportedTargets {
		for _, arch := range archs {
			metadata, e := getMetadata(target, arch)
			if e != nil {
				logger.Fatal("target: %s, arch: %s get metadata failed: %s", target, arch, e.Error())
			}

			logger.Debug("target: %s, arch: %s, release: %s", target, arch, metadata.Versioning.Release)
			logger.Debug("download url: %s", fmt.Sprintf(repositoryBinaryURL, target, arch, metadata.Versioning.Release))

			b, e := getArchive(target, arch, metadata.Versioning.Release)
			if e != nil {
				logger.Fatal("target: %s, arch: %s, release: %s get archive failed: %s",
					target, arch, metadata.Versioning.Release, e.Error())
			}

			fileName := fmt.Sprintf(releaseFileName, target, arch, metadata.Versioning.Release, "tar.xz")
			if e := os.WriteFile(filepath.Join(releaseDirName, fileName), b, os.ModePerm); e != nil {
				logger.Fatal("target: %s, arch: %s, release: %s write archive failed: %s",
					target, arch, metadata.Versioning.Release, e.Error())
			}

			data := map[string]string{
				"target":  target,
				"arch":    arch,
				"version": metadata.Versioning.Release,
				"sha256":  fmt.Sprintf("%x", sha256.Sum256(b)),
			}

			postgresGo := postgresTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
				if v, ok := data[tag]; ok {
					return w.Write([]byte(v))
				}

				if tg, ok := targetPath[target]; ok && tg != nil {
					if v, ok := tg[tag]; ok {
						return w.Write([]byte(v))
					}
				}

				return 0, fmt.Errorf("unsupported tag: %s", tag)
			})

			if e := os.WriteFile(filepath.Join(releaseDirName,
				fmt.Sprintf(releaseFileName, target, arch, metadata.Versioning.Release, "go")),
				[]byte(postgresGo), os.ModePerm); e != nil {
				logger.Fatal("target: %s, arch: %s, release: %s write postgres.go failed: %s",
					target, arch, metadata.Versioning.Release, e.Error())
			}

			logger.Info("target: %s, arch: %s, release: %s write archive success",
				target, arch, metadata.Versioning.Release)
		}
	}
}

// translate system target and architecture
func translateTargetAndArch(target, arch string) (string, string) {
	if v, ok := archAlias[arch]; ok {
		arch = v
	}

	return target, arch
}

// get release version of embedded postgres binaries
func getMetadata(target, arch string) (*MavenMetadata, error) {
	target, arch = translateTargetAndArch(target, arch)

	client := request.Get(fmt.Sprintf(repositoryMetadataURL,
		target, arch))

	if proxyAddr != "" {
		logger.Debug("set proxy: %s", proxyAddr)
		client = client.Proxy(proxyAddr)
	}

	res, e := client.End()
	if e != nil {
		return nil, fmt.Errorf("get metadata failed: %s", e.Error())
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("get metadata failed: %s", res.Status)
	}

	metadataBytes, e := res.Raw()
	if e != nil {
		return nil, fmt.Errorf("get metadata failed: %s", e.Error())
	}

	metadata := &MavenMetadata{}
	if e := xml.Unmarshal(metadataBytes, metadata); e != nil {
		return nil, fmt.Errorf("unmarshal metadata failed: %s", e.Error())
	}

	if metadata.ArtifactId != fmt.Sprintf(artifactIdTemplate, target, arch) {
		return nil, errors.New("get metadata failed: artifactId not match")
	}

	return metadata, nil
}

// get embedded postgres archive
func getArchive(target, arch, version string) ([]byte, error) {
	target, arch = translateTargetAndArch(target, arch)

	client := request.Get(fmt.Sprintf(repositoryBinaryURL,
		target, arch, version))

	if proxyAddr != "" {
		logger.Debug("set proxy: %s", proxyAddr)
		client = client.Proxy(proxyAddr)
	}

	res, e := client.End()
	if e != nil {
		return nil, fmt.Errorf("get archive failed: %s", e.Error())
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("get archive failed: %s", res.Status)
	}

	archiveBytes := bytes.NewBuffer(nil)

	downloader := &Downloader{
		Reader:         res.Body,
		ProgressLogger: logger.Progress(10, float64(res.ContentLength)/1024, "kb"),
	}

	fmt.Print("\r\n")

	if _, e := io.Copy(archiveBytes, downloader); e != nil {
		return nil, fmt.Errorf("copy failed: %s", e.Error())
	}

	zipReader, e := zip.NewReader(bytes.NewReader(archiveBytes.Bytes()), int64(archiveBytes.Len()))
	if e != nil {
		return nil, fmt.Errorf("extract archive failed: %s", e.Error())
	}

	for _, file := range zipReader.File {
		if !file.FileHeader.FileInfo().IsDir() && strings.HasSuffix(file.FileHeader.Name, ".txz") {
			reader, e := file.Open()
			if e != nil {
				return nil, fmt.Errorf("open content failed: %s", e.Error())
			}

			contentBytes, e := io.ReadAll(reader)
			if e != nil {
				return nil, fmt.Errorf("readall content failed: %s", e.Error())
			}

			return contentBytes, nil
		}
	}

	return nil, errors.New("extract archive failed: no txz file found")
}

type Downloader struct {
	io.Reader
	*logger.ProgressLogger
}

func (d *Downloader) Read(p []byte) (n int, e error) {
	n, e = d.Reader.Read(p)
	fmt.Printf("\033[1A\033[K")
	d.ProgressLogger.Append(float64(n)/1024, "downloading...")
	return
}
