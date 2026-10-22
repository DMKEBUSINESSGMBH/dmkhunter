package scanner

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"io/fs"
	"path/filepath"
	"regexp"
)

type Scanner struct {
	paths   []string
	Ignores *[]string
}

func NewScanner(paths []string, ignores *[]string) Scanner {
	return Scanner{
		paths:   paths,
		Ignores: ignores,
	}
}

// Walk calls fn for every regular file in the configured paths that is not ignored.
// Ignore patterns are matched against the path relative to the configured path, e.g. "var/log/dev.log".
// Files are passed as soon as they are found, so callers can start working before the walk is done.
func (s Scanner) Walk(fn func(f *model.File) error) error {
	ignores, err := s.compileIgnores()

	if err != nil {
		return err
	}

	for _, root := range s.paths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// a missing root is an error, unreadable entries below it are skipped
				if d == nil {
					return err
				}
				return nil
			}

			// symlinks are not regular files, so they are skipped here as well
			if !d.Type().IsRegular() {
				return nil
			}

			if pathIsIgnored(ignores, relativePath(root, path)) {
				return nil
			}

			return fn(model.NewFile(path))
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (s Scanner) Scan() map[string]*model.File {
	files := make(map[string]*model.File)

	s.Walk(func(f *model.File) error {
		files[f.Path] = f

		return nil
	})

	return files
}

func (s Scanner) compileIgnores() ([]*regexp.Regexp, error) {
	if s.Ignores == nil {
		return nil, nil
	}

	ignores := make([]*regexp.Regexp, 0, len(*s.Ignores))

	for _, pattern := range *s.Ignores {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		ignores = append(ignores, re)
	}

	return ignores, nil
}

// relativePath returns path relative to the scanned root with "/" separators, ignore patterns are matched against it.
func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)

	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}

// test if file matches ignore list
func pathIsIgnored(ignores []*regexp.Regexp, path string) bool {
	for _, re := range ignores {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
