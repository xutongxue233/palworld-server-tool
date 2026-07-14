package database

import (
	"encoding/json"
	"errors"
	"fmt"

	"go.etcd.io/bbolt"
)

var configBucket = []byte("config")

func ListConfigValues(db *bbolt.DB) (map[string]any, error) {
	values := make(map[string]any)
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(configBucket)
		if bucket == nil {
			return errors.New("config bucket does not exist")
		}
		return bucket.ForEach(func(key, raw []byte) error {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode config key %s: %w", key, err)
			}
			values[string(key)] = value
			return nil
		})
	})
	return values, err
}

func PutConfigValues(db *bbolt.DB, values map[string]any) error {
	if len(values) == 0 {
		return nil
	}
	encoded := make(map[string][]byte, len(values))
	for key, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("encode config key %s: %w", key, err)
		}
		encoded[key] = raw
	}
	return db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(configBucket)
		if bucket == nil {
			return errors.New("config bucket does not exist")
		}
		for key, raw := range encoded {
			if err := bucket.Put([]byte(key), raw); err != nil {
				return err
			}
		}
		return nil
	})
}
