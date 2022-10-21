package analyzer

import (
	"bytes"
	"fmt"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"go.etcd.io/bbolt"
)

type DatabaseAnalyzer struct {
	database *bbolt.DB
	name     string
}

func NewDatabaseAnalyzer(p string) (*DatabaseAnalyzer, error) {
	db, err := bbolt.Open(p, 0600, nil)
	defer db.Close()

	if err != nil {
		return nil, err
	}

	return &DatabaseAnalyzer{
		database: db,
		name:     p,
	}, nil
}

func (d DatabaseAnalyzer) Analyze(file model.File, stack *model.ViolationStack) error {
	hash, err := file.Hash()

	if err != nil {
		stack.Add(model.Violation{
			Severity: model.LEVEL_WARN,
			Message:  "Could not calculate hash",
			Filepath: file.Path,
		})

		return err
	}

	// TODO: find better way to not reopen db connection on every file
	d.database, err = bbolt.Open(d.name, 0600, nil)
	defer d.database.Close()
	if err != nil {
		return err
	}

	return d.database.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("file_hashes"))
		data := b.Get([]byte(file.Path))

		if data == nil {
			stack.Add(model.Violation{
				Filepath: file.Path,
				Message:  fmt.Sprintf("New file detected: %s", file.Path),
				Severity: model.LEVEL_ERROR,
			})

			return nil
		}

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

func (d DatabaseAnalyzer) Close() {
	d.database.Close()
}
