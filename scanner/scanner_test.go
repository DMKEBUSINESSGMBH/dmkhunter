package scanner

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"os"
	"path/filepath"
	"testing"
)

func TestScanner_Scan(t *testing.T) {
	scanner := Scanner{
		paths:   []string{"./testdata"},
		Ignores: &[]string{".*\\.tmp"},
	}

	expectedPaths := map[string]int{
		"testdata/dir1/LICENSE":      1,
		"testdata/dir1/.hiddenfile":  2,
		"testdata/dir2/file.txt":     3,
		"testdata/.hunter-test.conf": 4,
	}

	paths := scanner.Scan()

	if len(paths) == 0 {
		t.Errorf("scanner found no paths")
	}

	for expect, _ := range expectedPaths {
		path, ok := paths[expect]
		if ok != true {
			t.Fatalf("expected path %v not found", expect)
		}
		if (*path).Path != expect {
			t.Errorf("difference in path key and file path found: \nexp: %v\nfile:%v", expect, path.Path)
		}
	}

	for scanPath, _ := range paths {
		if _, exist := expectedPaths[scanPath]; !exist {
			t.Errorf("path '%v' was not expected, but found in directory", scanPath)
		}
	}
}

func TestScanner_IgnoresMatchRelativePath(t *testing.T) {
	tests := map[string][]string{
		// anchored to the scanned directory
		`^dir1/`: {"testdata/dir2/file.txt", "testdata/.hunter-test.conf", "testdata/invalid.tmp"},
		// files directly in the scanned directory only
		`^[^/]+$`: {"testdata/dir1/LICENSE", "testdata/dir1/.hiddenfile", "testdata/dir2/file.txt"},
		// absolute paths are not matched anymore
		`^testdata/`: {"testdata/dir1/LICENSE", "testdata/dir1/.hiddenfile", "testdata/dir2/file.txt", "testdata/.hunter-test.conf", "testdata/invalid.tmp"},
	}

	for pattern, expected := range tests {
		scanner := NewScanner([]string{"./testdata"}, &[]string{pattern})
		paths := scanner.Scan()

		if len(paths) != len(expected) {
			t.Errorf("%s: expected %d files, got %d: %v", pattern, len(expected), len(paths), paths)
			continue
		}
		for _, p := range expected {
			if _, ok := paths[p]; !ok {
				t.Errorf("%s: expected %s to be scanned", pattern, p)
			}
		}
	}
}

func TestRelativePath(t *testing.T) {
	tests := map[[2]string]string{
		{"/srv/project", "/srv/project/var/log/dev.log"}:  "var/log/dev.log",
		{"/srv/project/", "/srv/project/var/log/dev.log"}: "var/log/dev.log",
		{"./testdata", "testdata/dir1/LICENSE"}:           "dir1/LICENSE",
		{"testdata", "testdata/dir1/LICENSE"}:             "dir1/LICENSE",
		{"/", "/srv/project/index.php"}:                   "srv/project/index.php",
		{"/srv/pro", "/srv/project/index.php"}:            "../project/index.php",
		{"/srv/project", "/srv/project"}:                  ".",
		{"/srv/project", "/srv/project/../other/x.php"}:   "../other/x.php",
	}

	for in, expected := range tests {
		if got := relativePath(in[0], in[1]); got != expected {
			t.Errorf("relativePath(%q, %q): expected %q, got %q", in[0], in[1], expected, got)
		}
	}
}

func TestScanner_FollowsSymlinks(t *testing.T) {
	tmp := t.TempDir()

	mustMkdir := func(p string) {
		if err := os.MkdirAll(filepath.Join(tmp, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite := func(p string) {
		if err := os.WriteFile(filepath.Join(tmp, p), []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustLink := func(target, p string) {
		if err := os.Symlink(target, filepath.Join(tmp, p)); err != nil {
			t.Fatal(err)
		}
	}

	// project/ is scanned, outside/ is only reachable through links
	mustMkdir("project/sub")
	mustMkdir("outside/deep")
	mustWrite("project/index.php")
	mustWrite("outside/shared.php")
	mustWrite("outside/deep/lib.php")
	mustLink("../outside", "project/vendor")
	mustLink("../outside/shared.php", "project/shared.php")
	mustLink("missing.php", "project/broken.php")
	// loops back to project/ and to itself
	mustLink("..", "project/sub/up")
	mustLink("loop", "project/loop")
	mustLink("project", "rootlink")

	// shared.php links to vendor/shared.php, so it is only reported once
	expected := []string{
		"index.php",
		"vendor/shared.php",
		"vendor/deep/lib.php",
	}

	for _, root := range []string{filepath.Join(tmp, "project"), filepath.Join(tmp, "rootlink")} {
		paths := NewScanner([]string{root}, nil).Scan()

		if len(paths) != len(expected) {
			t.Errorf("%s: expected %d files, got %d: %v", root, len(expected), len(paths), paths)
			continue
		}
		for _, p := range expected {
			if _, ok := paths[filepath.Join(root, p)]; !ok {
				t.Errorf("%s: expected %s to be scanned", root, p)
			}
		}
	}

	// ignores match the path the file was found at, not the link target,
	// a file that is ignored under one path is still reported under a link that is not ignored
	root := filepath.Join(tmp, "project")
	paths := NewScanner([]string{root}, &[]string{`^vendor/`}).Scan()
	if len(paths) != 2 || paths[filepath.Join(root, "index.php")] == nil || paths[filepath.Join(root, "shared.php")] == nil {
		t.Errorf("expected index.php and shared.php, got %v", paths)
	}
}

func TestScanner_ReportsEveryFileOnce(t *testing.T) {
	tmp := t.TempDir()

	// real files are reported under their real path, even when a link to them sorts first
	for _, d := range []string{"src/lib", "outside"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"src/lib/a.php", "src/b.php", "outside/c.php"} {
		if err := os.WriteFile(filepath.Join(tmp, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{
		"src/a_lib":       "lib",
		"src/a_b.php":     "b.php",
		"src/lib/self":    ".",
		"src/c1":          "../outside",
		"src/c2":          "../outside",
		"src/c3.php":      "../outside/c.php",
		"outside/backref": "../src",
	}
	for p, target := range links {
		if err := os.Symlink(target, filepath.Join(tmp, p)); err != nil {
			t.Fatal(err)
		}
	}

	root := filepath.Join(tmp, "src")
	var found []string
	err := NewScanner([]string{root, filepath.Join(tmp, "src/lib")}, nil).Walk(func(f *model.File) error {
		rel, _ := filepath.Rel(root, f.Path)
		found = append(found, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"b.php", "lib/a.php", "c1/c.php"}
	if len(found) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, found)
	}
	for i := range expected {
		if found[i] != expected[i] {
			t.Errorf("expected %v, got %v", expected, found)
			break
		}
	}
}

func TestScanner_BrokenRootLinkIsAnError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "link")

	if err := os.Symlink("missing", root); err != nil {
		t.Fatal(err)
	}

	if err := NewScanner([]string{root}, nil).Walk(func(*model.File) error { return nil }); err == nil {
		t.Error("expected an error for a broken root link")
	}
}

func TestScanner_LinkInLinkedDirResolvesDotDotPhysically(t *testing.T) {
	tmp := t.TempDir()

	// project/vendor/up.php -> ../other/o.php means outside/../other/o.php, not project/other/o.php
	for _, d := range []string{"project/other", "outside", "other"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"project/other/o.php", "other/o.php"} {
		if err := os.WriteFile(filepath.Join(tmp, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../outside", filepath.Join(tmp, "project/vendor")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../other/o.php", filepath.Join(tmp, "outside/up.php")); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(tmp, "project")
	paths := NewScanner([]string{root}, nil).Scan()

	for _, p := range []string{"other/o.php", "vendor/up.php"} {
		if paths[filepath.Join(root, p)] == nil {
			t.Errorf("expected %s to be scanned, got %v", p, paths)
		}
	}
}
