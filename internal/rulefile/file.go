// Package rulefile reads bounded regular rule files without following symlinks.
package rulefile

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const MaxBytes = 1 << 20
const MaxFiles = 256
const MaxTotalBytes = 16 << 20

// Directory anchors child opens to one opened directory, even if its pathname
// is replaced during a reload. Callers own Close and do not reuse it afterward.
type Directory struct{ file *os.File }

func OpenDirectory(path string) (*Directory, error) {
	file, err := openDirectory(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return &Directory{file: file}, nil
}
func (d *Directory) Close() error                         { return d.file.Close() }
func (d *Directory) ReadDir(n int) ([]os.DirEntry, error) { return d.file.ReadDir(n) }
func (d *Directory) OpenDirectory(name string) (*Directory, error) {
	if !validChildName(name) {
		return nil, fmt.Errorf("rule group must be one path component: %q", name)
	}
	file, err := openChildDirectory(d.file, name)
	if err != nil {
		return nil, err
	}
	return &Directory{file: file}, nil
}
func (d *Directory) ReadFile(ctx context.Context, name string, maxBytes int) ([]byte, error) {
	if !validChildName(name) {
		return nil, fmt.Errorf("rule name must be one path component: %q", name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := openChildFile(d.file, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("rule must be a regular file: %s", name)
	}
	if info.Size() > int64(maxBytes) {
		return nil, fmt.Errorf("rule exceeds %d bytes: %s", maxBytes, name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// A growing regular file is still bounded even if fstat observed a small size.
	body, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(body) > maxBytes {
		return nil, fmt.Errorf("rule exceeds %d bytes: %s", maxBytes, name)
	}
	return body, nil
}

// Read rejects a symbolic-link parent directory or leaf and checks the opened
// descriptor's type. Nonblocking open prevents FIFO substitution from hanging.
func Read(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	directory, err := OpenDirectory(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	return directory.ReadFile(ctx, filepath.Base(path), MaxBytes)
}

func validChildName(name string) bool {
	return name != "." && name != ".." && name == filepath.Base(name)
}
