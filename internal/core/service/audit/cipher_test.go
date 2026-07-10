package audit

import "testing"

func TestCipherBox_SealOpenRoundtrip(t *testing.T) {
	box := newCipherBox(newFakeSecretStore())

	plaintext := []byte("hello world")
	ciphertext, err := box.seal(plaintext)
	if err != nil {
		t.Fatalf("seal() error = %v", err)
	}
	if string(ciphertext) == string(plaintext) {
		t.Fatal("seal() returned plaintext unchanged, want ciphertext")
	}

	got, err := box.open(ciphertext)
	if err != nil {
		t.Fatalf("open() error = %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("open() = %q, want %q", got, plaintext)
	}
}

// TestCipherBox_KeyReusedAcrossInstances confirms the key is
// loaded-or-created through SecretStore, not regenerated per process/
// instance -- otherwise every blob sealed before a restart (or by a
// concurrent capture-finalize goroutine racing key creation) would become
// permanently unreadable.
func TestCipherBox_KeyReusedAcrossInstances(t *testing.T) {
	secrets := newFakeSecretStore()

	box1 := newCipherBox(secrets)
	ciphertext, err := box1.seal([]byte("secret output"))
	if err != nil {
		t.Fatalf("seal() error = %v", err)
	}

	box2 := newCipherBox(secrets)
	got, err := box2.open(ciphertext)
	if err != nil {
		t.Fatalf("open() with a fresh cipherBox over the same store error = %v", err)
	}
	if string(got) != "secret output" {
		t.Fatalf("open() = %q, want %q", got, "secret output")
	}
}

func TestCipherBox_WrongKeyFailsToOpen(t *testing.T) {
	box1 := newCipherBox(newFakeSecretStore())
	ciphertext, err := box1.seal([]byte("secret output"))
	if err != nil {
		t.Fatalf("seal() error = %v", err)
	}

	box2 := newCipherBox(newFakeSecretStore()) // independent store -- generates a different key
	if _, err := box2.open(ciphertext); err == nil {
		t.Fatal("open() with a different key succeeded, want an error")
	}
}
