package database

import (
	"sync"
	"time"

	"github.com/zaigie/palworld-server-tool/internal/logger"
	"go.etcd.io/bbolt"
)

var db *bbolt.DB
var once sync.Once

func InitDB() *bbolt.DB {
	db_, err := bbolt.Open("pst.db", 0600, &bbolt.Options{Timeout: 1 * time.Minute})
	if err != nil {
		logger.Panic(err)
	}
	err = EnsureBuckets(db_)
	if err != nil {
		logger.Panic(err)
	}
	return db_
}

func EnsureBuckets(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		for _, name := range []string{
			"players",
			"guilds",
			"backups",
			"automation_tasks",
			"automation_runs",
			"automation_settings",
			"config",
		} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return err
			}
		}
		return nil
	})
}

func GetDB() *bbolt.DB {
	once.Do(func() {
		db = InitDB()
	})
	return db
}
