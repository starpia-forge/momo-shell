// Package localfs implements in.LocalFSUseCase directly against the local
// file system (os/filepath), mirroring how internal/core/service/transfer
// already handles the local side of uploads/downloads without a driven port.
package localfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// Service implements in.LocalFSUseCase.
type Service struct{}

var _ in.LocalFSUseCase = (*Service)(nil)

func New() *Service {
	return &Service{}
}

func toEntry(dir string, info os.FileInfo) domain.RemoteEntry {
	return domain.RemoteEntry{
		Name:     info.Name(),
		Path:     filepath.Join(dir, info.Name()),
		Size:     info.Size(),
		Mode:     uint32(info.Mode()),
		ModeText: info.Mode().String(),
		ModTime:  info.ModTime(),
		IsDir:    info.IsDir(),
	}
}

func (s *Service) ListDir(dir string) ([]domain.RemoteEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.RemoteEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			continue // vanished between readdir and stat -- skip rather than fail the whole listing
		}
		entries = append(entries, toEntry(dir, info))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func (s *Service) HomeDir() (string, error) {
	return os.UserHomeDir()
}

func (s *Service) Roots() ([]string, error) {
	if runtime.GOOS != "windows" {
		return []string{"/"}, nil
	}
	var roots []string
	for l := 'C'; l <= 'Z'; l++ {
		drive := fmt.Sprintf("%c:\\", l)
		if _, err := os.Stat(drive); err == nil {
			roots = append(roots, drive)
		}
	}
	return roots, nil
}

func (s *Service) Stat(path string) (domain.RemoteEntry, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.RemoteEntry{}, false, nil
	}
	if err != nil {
		return domain.RemoteEntry{}, false, err
	}
	return toEntry(filepath.Dir(path), info), true, nil
}

func (s *Service) Mkdir(path string) error {
	return os.Mkdir(path, 0o755)
}

func (s *Service) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (s *Service) Remove(path string) error {
	return os.Remove(path)
}

// Copy copies files (not directories) into dstDir, keeping their basenames.
// A source already located at its destination (pasting into the directory
// it's already in) is left untouched rather than truncated through itself.
func (s *Service) Copy(srcPaths []string, dstDir string) error {
	for _, src := range srcPaths {
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("copy: %s is a directory, not supported", src)
		}
		dst := filepath.Join(dstDir, filepath.Base(src))
		if samePath(src, dst) {
			continue
		}
		if err := copyFile(src, dst, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

// samePath compares two paths the way the local file system resolves them:
// case-insensitively on Windows (its paths are case-insensitive), verbatim
// elsewhere.
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// Move relocates files or directories into dstDir via os.Rename. A failed
// rename falls back to copy+remove for files (e.g. cross-volume moves on
// Windows); a failed cross-volume directory move is returned as-is.
func (s *Service) Move(srcPaths []string, dstDir string) error {
	for _, src := range srcPaths {
		dst := filepath.Join(dstDir, filepath.Base(src))
		if err := os.Rename(src, dst); err == nil {
			continue
		}

		info, statErr := os.Stat(src)
		if statErr != nil {
			return statErr
		}
		if info.IsDir() {
			return fmt.Errorf("move: cross-volume directory move not supported for %s", src)
		}
		if err := copyFile(src, dst, info.Mode()); err != nil {
			return err
		}
		if err := os.Remove(src); err != nil {
			return err
		}
	}
	return nil
}
