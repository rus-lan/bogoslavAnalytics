package artifact

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// withZeroUmask clears the process umask for the duration of the
// test and restores the previous value on cleanup. The mode
// assertions in this file check for an exact permission value; a
// nonzero ambient umask can only narrow what os.WriteFile/os.MkdirAll
// actually produce, so without this the tests would pass or fail
// depending on the environment they run in rather than on the code
// under test.
func withZeroUmask(t *testing.T) {
	t.Helper()
	old := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(old) })
}

// filePerm reads back the permission bits (ignoring file-type bits)
// of the file or directory at path.
func filePerm(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	return info.Mode().Perm()
}

// TestWriteFile_setsFileMode0600 checks that writeFile creates a new
// file at exactly 0600. The comparison is exact, not just "no world
// bit", so that reverting writeFile's mode back to 0o644 fails this
// test.
func TestWriteFile_setsFileMode0600(t *testing.T) {
	withZeroUmask(t)

	path := filepath.Join(t.TempDir(), "artifact.json")
	if err := writeFile(path, []byte(`{}`)); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	if got := filePerm(t, path); got != 0o600 {
		t.Errorf("file mode = %o, want %o", got, 0o600)
	}
}

// TestWriteFile_tightensPreExistingFileMode checks that rewriting a
// path that already held a file at a looser mode (as any artifact
// written before this package tightened its default would) still
// ends up at 0600. os.WriteFile's mode argument only applies to a
// file it creates, so writeFile must chmod explicitly to close that
// gap for already-written artifacts, not just brand new ones.
func TestWriteFile_tightensPreExistingFileMode(t *testing.T) {
	withZeroUmask(t)

	path := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(path, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if got := filePerm(t, path); got != 0o644 {
		t.Fatalf("seed file mode = %o, want %o", got, 0o644)
	}

	if err := writeFile(path, []byte(`{"new":true}`)); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	if got := filePerm(t, path); got != 0o600 {
		t.Errorf("file mode after rewrite = %o, want %o", got, 0o600)
	}
}

// TestWriteFile_createsDirectoryMode0700 checks that writeFile creates
// a missing parent directory at 0700 rather than 0755.
func TestWriteFile_createsDirectoryMode0700(t *testing.T) {
	withZeroUmask(t)

	dir := filepath.Join(t.TempDir(), "artifacts")
	path := filepath.Join(dir, "artifact.json")
	if err := writeFile(path, []byte(`{}`)); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	if got := filePerm(t, dir); got != 0o700 {
		t.Errorf("directory mode = %o, want %o", got, 0o700)
	}
}

// TestWriteFile_leavesPreExistingDirectoryModeAlone checks the other
// half of the directory decision: writeFile does not retroactively
// chmod a directory that already existed (os.MkdirAll is a no-op on
// an existing path and does not touch its mode). Unlike file
// contents, artifact filenames carry no personal data
// ("<kind>_<hash>.<ext>"), so there is no security reason to force an
// already-created directory to 0700 out from under whatever mode it
// currently has.
func TestWriteFile_leavesPreExistingDirectoryModeAlone(t *testing.T) {
	withZeroUmask(t)

	dir := filepath.Join(t.TempDir(), "artifacts")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("seed directory: %v", err)
	}

	path := filepath.Join(dir, "artifact.json")
	if err := writeFile(path, []byte(`{}`)); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	if got := filePerm(t, dir); got != 0o755 {
		t.Errorf("directory mode = %o, want %o (writeFile must not have touched it)", got, 0o755)
	}
}

// TestWrite_producesFileMode0600ForAllKindsAndFormats checks the
// public Write* entry points end to end: for every artifact kind and
// every format -- including text and html, which carry the same
// comment bodies and author names as json/yaml -- the file landing
// on disk is 0600.
func TestWrite_producesFileMode0600ForAllKindsAndFormats(t *testing.T) {
	withZeroUmask(t)

	formats := []Format{FormatJSON, FormatYAML, FormatText, FormatHTML}
	for _, k := range allKindCases() {
		for _, format := range formats {
			t.Run(k.name+"/"+string(format), func(t *testing.T) {
				ext, err := format.Extension()
				if err != nil {
					t.Fatalf("Extension() error = %v", err)
				}
				path := filepath.Join(t.TempDir(), k.name+"_test."+ext)

				if err := k.write(format, path); err != nil {
					t.Fatalf("write() error = %v", err)
				}

				if got := filePerm(t, path); got != 0o600 {
					t.Errorf("file mode = %o, want %o", got, 0o600)
				}
			})
		}
	}
}
