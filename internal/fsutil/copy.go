package fsutil

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyDirIfMissing copies the directory tree at src to dst, but only when src exists and dst
// doesn't. It reports whether it copied. The tree is copied into a temporary sibling first
// and renamed into place, so a failed copy leaves no dst behind. Only directories and regular
// files can be copied; anything else is an error.
func CopyDirIfMissing(src, dst string) (bool, error) {
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return false, nil
	}
	if _, err := os.Lstat(dst); err == nil || !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".copy-*")
	if err != nil {
		return false, err
	}
	if err := copyTree(src, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return false, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.RemoveAll(tmp)
		return false, err
	}
	return true, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			return copyFile(path, target)
		default:
			return fmt.Errorf("can't copy %s: not a file or directory", path)
		}
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
