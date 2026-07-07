package settings

import "testing"

type fakeRepo struct {
	data map[string]string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{data: make(map[string]string)}
}

func (f *fakeRepo) Get(key string) (string, bool, error) {
	v, ok := f.data[key]
	return v, ok, nil
}

func (f *fakeRepo) Set(key, value string) error {
	f.data[key] = value
	return nil
}

func (f *fakeRepo) All() (map[string]string, error) {
	out := make(map[string]string, len(f.data))
	for k, v := range f.data {
		out[k] = v
	}
	return out, nil
}

func TestGetAll_DefaultsWhenEmpty(t *testing.T) {
	svc := New(newFakeRepo())
	all, err := svc.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	want := map[string]string{KeyTheme: "dark", KeyAccent: "pink", KeyFontSize: "14", KeyScrollback: "10000"}
	for k, v := range want {
		if all[k] != v {
			t.Fatalf("GetAll()[%s] = %q, want %q", k, all[k], v)
		}
	}
}

func TestGetAll_MergesStoredOverDefaults(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo)
	if err := svc.Set(KeyTheme, "light"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	all, err := svc.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if all[KeyTheme] != "light" {
		t.Fatalf("GetAll()[theme] = %q, want light", all[KeyTheme])
	}
	if all[KeyFontSize] != "14" {
		t.Fatalf("GetAll()[fontSize] = %q, want default 14", all[KeyFontSize])
	}
}

func TestSet_UnknownKeyRejected(t *testing.T) {
	svc := New(newFakeRepo())
	if err := svc.Set("bogus.key", "x"); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestSet_ThemeRejectsInvalidValue(t *testing.T) {
	svc := New(newFakeRepo())
	if err := svc.Set(KeyTheme, "solarized"); err == nil {
		t.Fatal("expected error for invalid theme")
	}
}

func TestSet_ThemeAcceptsValidValues(t *testing.T) {
	svc := New(newFakeRepo())
	for _, v := range []string{"dark", "light"} {
		if err := svc.Set(KeyTheme, v); err != nil {
			t.Fatalf("Set(theme, %q) error = %v", v, err)
		}
	}
}

func TestSet_AccentRejectsInvalidValue(t *testing.T) {
	svc := New(newFakeRepo())
	if err := svc.Set(KeyAccent, "chartreuse"); err == nil {
		t.Fatal("expected error for invalid accent")
	}
}

func TestSet_AccentAcceptsValidValues(t *testing.T) {
	svc := New(newFakeRepo())
	for _, v := range []string{"pink", "orange", "purple", "blue"} {
		if err := svc.Set(KeyAccent, v); err != nil {
			t.Fatalf("Set(accent, %q) error = %v", v, err)
		}
	}
}

func TestSet_FontSizeRejectsNonInteger(t *testing.T) {
	svc := New(newFakeRepo())
	if err := svc.Set(KeyFontSize, "large"); err == nil {
		t.Fatal("expected error for non-integer font size")
	}
}

func TestSet_FontSizeBoundaries(t *testing.T) {
	svc := New(newFakeRepo())
	if err := svc.Set(KeyFontSize, "8"); err != nil {
		t.Fatalf("Set(fontSize, 8) error = %v", err)
	}
	if err := svc.Set(KeyFontSize, "32"); err != nil {
		t.Fatalf("Set(fontSize, 32) error = %v", err)
	}
	if err := svc.Set(KeyFontSize, "7"); err == nil {
		t.Fatal("expected error for fontSize below minimum")
	}
	if err := svc.Set(KeyFontSize, "33"); err == nil {
		t.Fatal("expected error for fontSize above maximum")
	}
}

func TestSet_ScrollbackBoundaries(t *testing.T) {
	svc := New(newFakeRepo())
	if err := svc.Set(KeyScrollback, "1000"); err != nil {
		t.Fatalf("Set(scrollback, 1000) error = %v", err)
	}
	if err := svc.Set(KeyScrollback, "100000"); err != nil {
		t.Fatalf("Set(scrollback, 100000) error = %v", err)
	}
	if err := svc.Set(KeyScrollback, "999"); err == nil {
		t.Fatal("expected error for scrollback below minimum")
	}
	if err := svc.Set(KeyScrollback, "100001"); err == nil {
		t.Fatal("expected error for scrollback above maximum")
	}
}
