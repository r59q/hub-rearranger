// Package bootstrap packages reviewed repository artifacts without executing
// their contents, copying authentication, or retaining a second template tree.
package bootstrap

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

const maxFileBytes = domain.BootstrapMaxBytes

var ErrUnsafeFile = errors.New("bootstrap requires bounded regular UTF-8 files without symlink paths")

// ReadFile confines access to an opened root and rejects symlink components,
// directories, binary content and oversized files, including in target snapshots.
func ReadFile(root *os.Root, path string) (string, error) {
	parts := strings.Split(path, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", ErrUnsafeFile
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", ErrUnsafeFile
		}
		if i == len(parts)-1 && (!info.Mode().IsRegular() || info.Size() > maxFileBytes) {
			return "", ErrUnsafeFile
		}
	}
	file, err := root.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return "", err
	}
	if len(content) > maxFileBytes || !utf8.Valid(content) || strings.ContainsRune(string(content), '\x00') {
		return "", ErrUnsafeFile
	}
	return string(content), nil
}

// Snapshot only reads paths present in the canonical package. Missing files
// are additions; unsupported existing paths fail rather than being overwritten.
func Snapshot(root *os.Root, templates map[string]string) (map[string]string, error) {
	files := map[string]string{}
	for path := range templates {
		content, err := ReadFile(root, path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, ErrUnsafeFile
		}
		files[path] = content
	}
	return files, nil
}
