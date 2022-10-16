package scanner

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"io/fs"
	"log"
	"os"
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

func (s Scanner) Scan() map[string]*model.File {
	files := make(map[string]*model.File)

	for _, path := range s.paths {
		filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() || d.Type().IsRegular() == false {
				return nil
			}

			if s.pathIsIgnored(path) {
				return nil
			}

			// test for symlinks
			newPath, err := evaluateSymLink(path)
			if err != nil {
				return err
			}
			if len(newPath) > 0 {
				path = newPath
			}
			f, err := model.NewFile(path)

			if err != nil {
				return err
			}

			files[path] = f

			return nil
		})
	}

	return files
}

func evaluateSymLink(path string) (string, error) {
	fileInfo, err := os.Stat(path)

	if err != nil {
		return "", err
	}

	if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		evalpath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		evalfileStat, err := os.Stat(evalpath)

		if err != nil || evalfileStat.IsDir() {
			return "", err
		}
		path = evalpath
	}

	return "", nil
}

// test if file matches ignore list
func (s Scanner) pathIsIgnored(path string) bool {
	if s.Ignores == nil {
		return false
	}

	for _, pattern := range *s.Ignores {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatal(err)
		}
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
