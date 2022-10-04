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
		"testdata/dir2/go.mod":       3,
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

		t.Logf("found %v", path)
	}

	for scanPath, _ := range paths {
		if _, exist := expectedPaths[scanPath]; !exist {
			t.Errorf("path '%v' was not expected, but found in directory", scanPath)
		}
	}
}
