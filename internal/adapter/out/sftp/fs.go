// Package sftp implements out.RemoteFileSystem over an SFTP subsystem
// channel opened on an existing *ssh.Client (via sftp.NewClient). It knows
// nothing about sshconn -- the *ssh.Client crosses the boundary through a
// factory function injected into sshconn.New, so the two adapters never
// import each other.
package sftp

import (
	"io"
	"os"
	"path"

	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

const maxPacketSize = 32 * 1024

// fs wraps a *pkgsftp.Client to satisfy out.RemoteFileSystem.
type fs struct {
	client *pkgsftp.Client
}

var _ out.RemoteFileSystem = (*fs)(nil)

// NewFromClient opens an SFTP subsystem channel on client. Passed to
// sshconn.New so it can be invoked lazily, per session, without sshconn
// importing this package's concrete type.
func NewFromClient(client *ssh.Client) (out.RemoteFileSystem, error) {
	c, err := pkgsftp.NewClient(client, pkgsftp.MaxPacket(maxPacketSize), pkgsftp.UseConcurrentWrites(true))
	if err != nil {
		return nil, err
	}
	return &fs{client: c}, nil
}

func toRemoteEntry(dir string, info os.FileInfo) domain.RemoteEntry {
	return domain.RemoteEntry{
		Name:     info.Name(),
		Path:     path.Join(dir, info.Name()),
		Size:     info.Size(),
		Mode:     uint32(info.Mode()),
		ModeText: info.Mode().String(),
		ModTime:  info.ModTime(),
		IsDir:    info.IsDir(),
	}
}

func (f *fs) ReadDir(dir string) ([]domain.RemoteEntry, error) {
	infos, err := f.client.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.RemoteEntry, len(infos))
	for i, info := range infos {
		entries[i] = toRemoteEntry(dir, info)
	}
	return entries, nil
}

func (f *fs) Stat(p string) (domain.RemoteEntry, error) {
	info, err := f.client.Stat(p)
	if err != nil {
		return domain.RemoteEntry{}, err
	}
	return toRemoteEntry(path.Dir(p), info), nil
}

func (f *fs) Open(p string) (io.ReadCloser, error) {
	return f.client.Open(p)
}

func (f *fs) Create(p string) (io.WriteCloser, error) {
	return f.client.Create(p)
}

func (f *fs) Mkdir(p string) error {
	return f.client.Mkdir(p)
}

func (f *fs) Rename(oldPath, newPath string) error {
	return f.client.Rename(oldPath, newPath)
}

func (f *fs) Remove(p string) error {
	info, err := f.client.Stat(p)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return f.client.RemoveDirectory(p)
	}
	return f.client.Remove(p)
}

func (f *fs) Chmod(p string, mode uint32) error {
	return f.client.Chmod(p, os.FileMode(mode))
}

func (f *fs) HomeDir() (string, error) {
	return f.client.RealPath(".")
}

func (f *fs) Close() error {
	return f.client.Close()
}
