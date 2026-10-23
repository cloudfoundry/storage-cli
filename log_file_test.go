package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogFilePermissionsArePrivateForNewAndExistingFiles(t *testing.T) {
	for _, existing := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "logs", "storage.log")
		if existing {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("old-log\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0644); err != nil {
				t.Fatal(err)
			}
		}
		file := createOrUseProvided(path)
		if _, err := file.WriteString("new-log\n"); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		stat, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if stat.Mode().Perm() != 0600 {
			t.Fatalf("log mode %o", stat.Mode().Perm())
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := "new-log\n"
		if existing {
			want = "old-log\n" + want
		}
		if string(contents) != want {
			t.Fatal("log contents changed")
		}
	}
}
