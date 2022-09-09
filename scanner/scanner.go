package scanner

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"log"
	"path/filepath"
)

type Scanner struct {
	paths []string
}

func NewScanner(paths []string) Scanner {
	return Scanner{
		paths: paths,
	}
}

func (s Scanner) Scan() map[string]*model.File {
	files := make(map[string]*model.File)

	for _, path := range s.paths {
		matches, err := filepath.Glob(path)

		if err != nil {
			log.Fatal(err)
		}

		for _, p := range matches {
			f, err := model.NewFile(p)

			if err != nil {
				continue
			}

			files[p] = f
		}
	}

	return files
}
