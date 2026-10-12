package gif

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildGIF(t *testing.T) {
	out := filepath.Join(t.TempDir(), "demo.gif")
	if err := Build("phy131", out); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() < 20_000 {
		t.Errorf("GIF too small: %d bytes", fi.Size())
	}
	fh, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	head := make([]byte, 6)
	if _, err := fh.Read(head); err != nil {
		t.Fatal(err)
	}
	if string(head) != "GIF89a" && string(head) != "GIF87a" {
		t.Errorf("not a GIF: %q", head)
	}
}

func TestBuildGIFUnknownSheet(t *testing.T) {
	out := filepath.Join(t.TempDir(), "demo.gif")
	if err := Build("nope101", out); err == nil {
		t.Error("expected error for unknown sheet")
	}
}
