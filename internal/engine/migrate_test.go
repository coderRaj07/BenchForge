package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSQLFilesSortsAndSkipsNonSQL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "002_b.sql"), []byte("select 2;"), 0o644)
	os.WriteFile(filepath.Join(dir, "001_a.sql"), []byte("select 1;"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("notes"), 0o644)
	files, err := sqlFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || filepath.Base(files[0]) != "001_a.sql" || filepath.Base(files[1]) != "002_b.sql" {
		t.Fatalf("files = %v", files)
	}
}
