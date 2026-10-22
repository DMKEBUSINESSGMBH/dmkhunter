package scanner

import (
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
	}

	for in, expected := range tests {
		if got := relativePath(in[0], in[1]); got != expected {
			t.Errorf("relativePath(%q, %q): expected %q, got %q", in[0], in[1], expected, got)
		}
	}
}
