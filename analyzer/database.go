package analyzer

import (
	"bytes"
	"fmt"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"go.etcd.io/bbolt"
)

type DatabaseAnalyzer struct {
	database *bbolt.DB
}

func NewDatabaseAnalzer(p string) (*DatabaseAnalyzer, error) {
	db, err := bbolt.Open(p, 0600, nil)
	defer db.Close()

	if err != nil {
		return nil, err
	}

	return &DatabaseAnalyzer{
		database: db,
	}, nil
}

func (d DatabaseAnalyzer) Analyze(file model.File, stack model.ViolationStack) error {
	hash, err := file.Hash()

	if err != nil {
		stack.Add(model.Violation{
			Severity: model.LEVEL_WARN,
			Message:  "Could not calculate hash",
			Filepath: file.Path,
		})

		return err
	}

	return d.database.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("file_hashes"))
		data := b.Get([]byte(file.Path))

		if 0 != bytes.Compare(data, hash) {
			stack.Add(model.Violation{
				Filepath: file.Path,
				Message:  fmt.Sprintf("The file %s has been changed!", file.Path),
				Severity: model.LEVEL_ERROR,
			})
		}

		return nil
	})
}
