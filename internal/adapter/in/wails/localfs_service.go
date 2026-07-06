package wails

import "momo-shell/internal/core/port/in"

// LocalFSService is the Wails-bound facade over in.LocalFSUseCase. It only
// converts between JSON-facing DTOs and domain/port types -- no business logic.
type LocalFSService struct {
	uc in.LocalFSUseCase
}

func NewLocalFSService(uc in.LocalFSUseCase) *LocalFSService {
	return &LocalFSService{uc: uc}
}

func (s *LocalFSService) ListDir(path string) ([]RemoteEntryDTO, error) {
	entries, err := s.uc.ListDir(path)
	if err != nil {
		return nil, err
	}
	dtos := make([]RemoteEntryDTO, len(entries))
	for i, e := range entries {
		dtos[i] = remoteEntryToDTO(e)
	}
	return dtos, nil
}

func (s *LocalFSService) HomeDir() (string, error) {
	return s.uc.HomeDir()
}

func (s *LocalFSService) Roots() ([]string, error) {
	return s.uc.Roots()
}

// Stat returns nil (not an error) when path doesn't exist -- Wails bindings
// only support (value, error) returns, so the "found" flag from
// in.LocalFSUseCase.Stat is folded into nil-vs-non-nil here.
func (s *LocalFSService) Stat(path string) (*RemoteEntryDTO, error) {
	entry, ok, err := s.uc.Stat(path)
	if err != nil || !ok {
		return nil, err
	}
	dto := remoteEntryToDTO(entry)
	return &dto, nil
}

func (s *LocalFSService) Mkdir(path string) error {
	return s.uc.Mkdir(path)
}

func (s *LocalFSService) Rename(oldPath, newPath string) error {
	return s.uc.Rename(oldPath, newPath)
}

func (s *LocalFSService) Remove(path string) error {
	return s.uc.Remove(path)
}

func (s *LocalFSService) Copy(srcPaths []string, dstDir string) error {
	return s.uc.Copy(srcPaths, dstDir)
}

func (s *LocalFSService) Move(srcPaths []string, dstDir string) error {
	return s.uc.Move(srcPaths, dstDir)
}
