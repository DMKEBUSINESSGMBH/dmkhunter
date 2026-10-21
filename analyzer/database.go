package analyzer

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"go.etcd.io/bbolt"
)

var bucketName = []byte("file_hashes")

// DatabaseAnalyzer keeps the database open until Close, it is safe for concurrent use.
type DatabaseAnalyzer struct {
	database *bbolt.DB
}

func NewDatabaseAnalyzer(p string) (*DatabaseAnalyzer, error) {
	db, err := bbolt.Open(p, 0600, &bbolt.Options{ReadOnly: true, Timeout: 5 * time.Second})

	if err != nil {
		return nil, fmt.Errorf("could not open database %s: %w", p, err)
	}

	err = db.View(func(tx *bbolt.Tx) error {
		if tx.Bucket(bucketName) == nil {
			return errors.New("no file hashes found, run rescan first")
		}
		return nil
	})

	if err != nil {
		db.Close()
		return nil, fmt.Errorf("database %s: %w", p, err)
	}

	return &DatabaseAnalyzer{
		database: db,
	}, nil
}

func (d *DatabaseAnalyzer) Analyze(file model.File, stack *model.ViolationStack) error {
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
		data := tx.Bucket(bucketName).Get([]byte(file.Path))

		if data == nil {
			stack.Add(model.Violation{
				Filepath: file.Path,
				Message:  fmt.Sprintf("New file detected: %s", filepath.Base(file.Path)),
				Severity: model.LEVEL_ERROR,
			})

			return nil
		}

		if !bytes.Equal(data, hash) {
			stack.Add(model.Violation{
				Filepath: file.Path,
				Message:  fmt.Sprintf("The file %s has been changed!", filepath.Base(file.Path)),
				Severity: model.LEVEL_ERROR,
			})
		}

		return nil
	})
}

func (d *DatabaseAnalyzer) Close() error {
	return d.database.Close()
}
