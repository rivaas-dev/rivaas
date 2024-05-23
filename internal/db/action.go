package db

import (
	"errors"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DBClient represents database client.
type DBClient struct {
	client *gorm.DB
}

// New constructs a new database client.
func New(host string, port uint16, username string, password string, database string) (*DBClient, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		host,
		username,
		password,
		database,
		port,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	return &DBClient{
		client: db,
	}, err
}

// GetKey returns a key by hash.
func (s DBClient) GetKey(hash string) (*Key, error) {
	var row Key
	res := s.client.First(&row, "id = ? AND deleted_at is NULL", hash)
	if res.Error != nil && errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil // not really an error, just no result
	}
	return &row, res.Error
}

// GetKeys gets all the keys with optional filters.
func (s DBClient) GetKeys(actorID, description, customerID, accountID string) ([]*Key, error) {
	var keyList []*Key
	q := s.client

	if actorID != "" {
		searchKey := Key{ActorID: actorID}
		q = q.Where(searchKey)
	}
	if description != "" {
		q = q.Where("description ILIKE ?", fmt.Sprintf("%%%s%%", description))
	}
	if customerID != "" || accountID != "" {
		q = q.Where("actor_id ILIKE ?", fmt.Sprintf("%%%s%%%s%%", customerID, accountID))
	}
	q = q.Where("deleted_at is NULL")

	result := q.Find(&keyList)
	if result.Error != nil {
		return nil, result.Error
	}
	return keyList, nil
}
