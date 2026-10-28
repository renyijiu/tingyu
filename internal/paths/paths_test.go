package paths

import (
	"path/filepath"
	"testing"
)

func TestDirOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "data")
	t.Setenv("TINGYU_DATA_DIR", want)
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDirDefaultEndsWithTingyu(t *testing.T) {
	t.Setenv("TINGYU_DATA_DIR", "")
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "Tingyu" {
		t.Fatalf("got %q", got)
	}
}
