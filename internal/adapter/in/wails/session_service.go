package wails

import (
	"encoding/base64"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/in"
)

// LocalSessionOpts is the JSON-facing request DTO for CreateLocalSession.
type LocalSessionOpts struct {
	Shell string `json:"shell"`
	Cwd   string `json:"cwd"`
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
}

// SessionInfoDTO is the JSON-facing response DTO for session queries.
type SessionInfoDTO struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Shell string `json:"shell"`
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
}

// SessionService is the Wails-bound facade over in.SessionUseCase. It only
// converts between JSON-facing DTOs and domain types -- no business logic.
type SessionService struct {
	uc in.SessionUseCase
}

func NewSessionService(uc in.SessionUseCase) *SessionService {
	return &SessionService{uc: uc}
}

func (s *SessionService) CreateLocalSession(opts LocalSessionOpts) (SessionInfoDTO, error) {
	info, err := s.uc.CreateLocal(in.LocalOpts{
		Shell: opts.Shell,
		Cwd:   opts.Cwd,
		Cols:  opts.Cols,
		Rows:  opts.Rows,
	})
	if err != nil {
		return SessionInfoDTO{}, err
	}
	return toDTO(info), nil
}

func (s *SessionService) WriteSession(id string, dataB64 string) error {
	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return err
	}
	return s.uc.Write(id, data)
}

func (s *SessionService) ResizeSession(id string, cols, rows int) error {
	return s.uc.Resize(id, cols, rows)
}

func (s *SessionService) CloseSession(id string) error {
	return s.uc.Close(id)
}

func toDTO(info domain.SessionInfo) SessionInfoDTO {
	return SessionInfoDTO{
		ID:    info.ID,
		Kind:  string(info.Kind),
		Shell: info.Shell,
		Cols:  info.Cols,
		Rows:  info.Rows,
	}
}
