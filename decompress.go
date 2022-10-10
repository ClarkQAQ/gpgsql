package gpgsql

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ClarkQAQ/gpgsql/release"

	"github.com/xi2/xz"
)

var (
	binaryRootPath string = func() string {
		base, _ := os.UserCacheDir()

		if strings.TrimSpace(base) == "" {
			base = os.TempDir()
		}

		return filepath.Join(base, "gpgsql",
			fmt.Sprintf("%s_%s", release.Version, release.Sha256[:8]))
	}()
)

// form https://github.com/fergusstrange/embedded-postgres/blob/master/decompression.go#L23
func DecompressBinary(force bool) error {
	xzReader, e := xz.NewReader(bytes.NewReader(release.Archive), 0)
	if e != nil {
		return fmt.Errorf("decompress archive failed: %s", e.Error())
	}

	if f, _ := os.Stat(binaryRootPath); (f != nil && f.IsDir()) && !force {
		return nil
	}

	if e := os.RemoveAll(binaryRootPath); e != nil {
		return fmt.Errorf("remove binary root path failed: %s", e.Error())
	}

	if e := os.MkdirAll(binaryRootPath, os.ModePerm); e != nil {
		return fmt.Errorf("create temp dir failed: %s", e.Error())
	}

	tarReader := tar.NewReader(xzReader)

	for {
		header, e := tarReader.Next()

		if errors.Is(e, io.EOF) {
			return nil
		}

		if e != nil {
			return fmt.Errorf("read archive header failed: %s", e.Error())
		}

		targetPath := filepath.Join(binaryRootPath, header.Name)

		if e := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); e != nil {
			return fmt.Errorf("create directory failed: %s", e.Error())
		}

		if e := func() (e error) {
			switch header.Typeflag {
			case tar.TypeReg:
				outFile, e := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				if e != nil {
					return fmt.Errorf("create file failed: %s", e.Error())
				}

				defer func() {
					if e = outFile.Close(); e != nil {
						e = fmt.Errorf("close file failed: %s", e.Error())
					}
				}()

				if _, e = io.Copy(outFile, tarReader); e != nil {
					return fmt.Errorf("write file failed: %s", e.Error())
				}
			case tar.TypeSymlink:
				if e = os.RemoveAll(targetPath); e != nil {
					return fmt.Errorf("remove symlink failed: %s", e.Error())
				}

				if e = os.Symlink(header.Linkname, targetPath); e != nil {
					return fmt.Errorf("create symlink failed: %s", e.Error())
				}
			}

			return nil
		}(); e != nil {
			return e
		}
	}
}

func CleanBinary() error {
	if f, _ := os.Stat(binaryRootPath); f != nil {
		return os.RemoveAll(binaryRootPath)
	}

	return nil
}
