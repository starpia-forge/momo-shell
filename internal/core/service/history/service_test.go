package history

import (
	"errors"
	"sync"
	"testing"

	"momo-shell/internal/core/domain"
)

var errFakeNotFound = errors.New("fake: not found")

type fakeHistoryRepo struct {
	mu      sync.Mutex
	entries []domain.HistoryEntry
	nextID  int64
}

func (f *fakeHistoryRepo) Append(hostID *string, command string, executedAt int64) (domain.HistoryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	e := domain.HistoryEntry{ID: f.nextID, HostID: hostID, Command: command, ExecutedAt: executedAt}
	f.entries = append(f.entries, e)
	return e, nil
}

func (f *fakeHistoryRepo) TouchLast(id int64, executedAt int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.entries {
		if f.entries[i].ID == id {
			f.entries[i].ExecutedAt = executedAt
			return nil
		}
	}
	return errFakeNotFound
}

func (f *fakeHistoryRepo) LastForHost(hostID *string) (domain.HistoryEntry, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var best *domain.HistoryEntry
	for i := range f.entries {
		e := &f.entries[i]
		if !sameHostID(e.HostID, hostID) {
			continue
		}
		if best == nil || e.ExecutedAt > best.ExecutedAt {
			best = e
		}
	}
	if best == nil {
		return domain.HistoryEntry{}, false, nil
	}
	return *best, true, nil
}

func (f *fakeHistoryRepo) List(_ domain.HistoryQuery) ([]domain.HistoryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.HistoryEntry, len(f.entries))
	copy(out, f.entries)
	return out, nil
}

func (f *fakeHistoryRepo) Delete(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.entries {
		if e.ID == id {
			f.entries = append(f.entries[:i], f.entries[i+1:]...)
			return nil
		}
	}
	return errFakeNotFound
}

func (f *fakeHistoryRepo) Clear(_ *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = nil
	return nil
}

func sameHostID(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

type fakePublisher struct {
	mu       sync.Mutex
	payloads []HistoryAppendedPayload
}

func (f *fakePublisher) Publish(_ string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := payload.(HistoryAppendedPayload); ok {
		f.payloads = append(f.payloads, p)
	}
}

func newTestService() (*Service, *fakeHistoryRepo, *fakePublisher) {
	repo := &fakeHistoryRepo{}
	pub := &fakePublisher{}
	return New(repo, pub), repo, pub
}

func TestService_OnInput_PersistsCommittedLine(t *testing.T) {
	svc, repo, pub := newTestService()
	svc.Attach("s1", "")
	svc.OnInput("s1", []byte("ls -la\r"))
	svc.Close()

	if len(repo.entries) != 1 || repo.entries[0].Command != "ls -la" {
		t.Fatalf("entries = %+v, want one 'ls -la' entry", repo.entries)
	}
	if len(pub.payloads) != 1 || pub.payloads[0].Command != "ls -la" {
		t.Fatalf("payloads = %+v, want one 'ls -la' payload", pub.payloads)
	}
}

func TestService_OnInput_ConsecutiveDuplicate_BumpsTimestampInstead(t *testing.T) {
	svc, repo, _ := newTestService()
	svc.Attach("s1", "")
	svc.OnInput("s1", []byte("ls -la\r"))
	svc.OnInput("s1", []byte("ls -la\r"))
	svc.Close()

	if len(repo.entries) != 1 {
		t.Fatalf("expected exactly one row after a consecutive duplicate, got %+v", repo.entries)
	}
}

func TestService_OnInput_MaskedCommand_NeverSaved(t *testing.T) {
	svc, repo, pub := newTestService()
	svc.Attach("s1", "")
	svc.OnInput("s1", []byte("export DB_PASSWORD=hunter2\r"))
	svc.Close()

	if len(repo.entries) != 0 {
		t.Fatalf("expected masked command to be skipped, got %+v", repo.entries)
	}
	if len(pub.payloads) != 0 {
		t.Fatalf("expected no publish for a masked command, got %+v", pub.payloads)
	}
}

func TestService_OnOutput_AltScreen_SuppressesCapture(t *testing.T) {
	svc, repo, _ := newTestService()
	svc.Attach("s1", "")

	svc.OnOutput("s1", []byte("\x1b[?1049h")) // e.g. vim enters the alt screen
	svc.OnInput("s1", []byte("this should not be captured\r"))
	svc.OnOutput("s1", []byte("\x1b[?1049l")) // exits
	svc.OnInput("s1", []byte("ls\r"))
	svc.Close()

	if len(repo.entries) != 1 || repo.entries[0].Command != "ls" {
		t.Fatalf("entries = %+v, want only 'ls' captured after alt-screen exit", repo.entries)
	}
}

func TestService_Detach_StopsCapture(t *testing.T) {
	svc, repo, _ := newTestService()
	svc.Attach("s1", "")
	svc.Detach("s1")
	svc.OnInput("s1", []byte("ls -la\r")) // no recorder anymore -- ignored
	svc.Close()

	if len(repo.entries) != 0 {
		t.Fatalf("expected no entries after detach, got %+v", repo.entries)
	}
}

func TestService_List_Delete_DelegateToRepo(t *testing.T) {
	svc, repo, _ := newTestService()
	svc.Attach("s1", "")
	svc.OnInput("s1", []byte("ls\r"))
	svc.Close()

	entries, err := svc.List(domain.HistoryQuery{})
	if err != nil || len(entries) != 1 {
		t.Fatalf("List() = %+v, err=%v", entries, err)
	}

	if err := svc.Delete(entries[0].ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(repo.entries) != 0 {
		t.Fatalf("expected repo empty after Delete, got %+v", repo.entries)
	}
}
