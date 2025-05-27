package db

import (
	"errors"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ErrKeyNotFound = errors.New("key not found")

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
		return nil, ErrKeyNotFound
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

// GetKeysPaginated gets all the keys with optional filters and paginates them.
func (s DBClient) GetKeysPaginated(searchParams SearchParams, pageSize uint16, pageNumber uint32) (keys []*Key, totalResults int64, err error) {
	var keyList []*Key
	q := s.client

	actorIDQuery, actorIDArgs := createActorIDQuery(searchParams.ActorID, searchParams.CustomerID, searchParams.AccountID)
	if actorIDQuery != "" {
		q = q.Where(actorIDQuery, actorIDArgs)
	}

	if searchParams.Description != "" {
		q = q.Where("description ILIKE ?", fmt.Sprintf("%%%s%%", searchParams.Description))
	}

	q = q.Where("deleted_at is NULL")

	err = q.Model(new(Key)).Count(&totalResults).Error
	if err != nil {
		return nil, 0, errors.New("error on calculating the totalResults")
	}

	err = q.Scopes(Paginate(uint(pageNumber), uint(pageSize))).
		Order("creation_date DESC").
		Find(&keyList).
		Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, errors.New("key was not found")
	}

	return keyList, totalResults, nil
}

// Paginate paginates the result
func Paginate(pageNumber, pageSize uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset := (pageNumber - 1) * pageSize
		return db.Offset(int(offset)).Limit(int(pageSize))
	}
}

func createActorIDQuery(actorID, customerID, accountID string) (query string, args []any) {
	if actorID != "" {
		return "actor_id = ?", []any{actorID}
	}

	var actorIDArg string
	switch {
	case customerID != "" && accountID != "":
		actorIDArg = fmt.Sprintf("urn:api:key:%s:%s:%%", customerID, accountID)
	case customerID != "":
		actorIDArg = fmt.Sprintf("urn:api:key:%s:%%:%%", customerID)
	case accountID != "":
		actorIDArg = fmt.Sprintf("urn:api:key:%%:%s:%%", accountID)
	}
	return "actor_id ILIKE ?", []any{actorIDArg}
}
