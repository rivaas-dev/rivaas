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
	res := s.client.First(&row, "id = ?", hash)
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
func (s DBClient) GetKeysPaginated(searchParams SearchParams, pageSize uint, pageNumber uint) (keys []*Key, totalResults int64, err error) {
	var keyList []*Key
	query := s.prepareListQuery(searchParams)

	err = query.Model(new(Key)).Count(&totalResults).Error
	if err != nil {
		return nil, 0, errors.New("error on calculating the totalResults")
	}

	err = query.Scopes(Paginate(pageNumber, pageSize)).
		Order("creation_date DESC").
		Find(&keyList).
		Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, errors.New("key was not found")
	}

	return keyList, totalResults, nil
}

// GetKeysCountPerEnvironment gets number of all the keys with optional filters per environment.
func (s DBClient) GetKeysCountPerEnvironment(searchParams SearchParams) (totalResults map[Environment]int64, err error) {
	query := s.prepareListQuery(searchParams)
	query = query.Select("environment, count(*) as count").Table(Key{}.TableName()).Group("environment")

	// we need this intermediate struct because can't handle maps
	type EnvCount struct {
		Environment string `gorm:"column:environment"`
		Count       int64  `gorm:"column:count"`
	}
	var envCounts []EnvCount
	err = query.Scan(&envCounts).Error
	if err != nil {
		return nil, fmt.Errorf("calculating the number of keys by environment: %w", err)
	}
	counts := make(map[Environment]int64, len(envCounts))
	for _, envCount := range envCounts {
		counts[envCount.Environment] = envCount.Count
	}
	return counts, nil
}

// Paginate paginates the result
func Paginate(pageNumber, pageSize uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset := (pageNumber - 1) * pageSize
		return db.Offset(int(offset)).Limit(int(pageSize))
	}
}

func createActorIDQuery(actorID, customerID, accountID string) (query string, args []any) {
	if actorID == "" && customerID == "" && accountID == "" {
		return "", []any{}
	}

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

func (s DBClient) prepareListQuery(searchParams SearchParams) *gorm.DB {
	q := s.client

	actorIDQuery, actorIDArgs := createActorIDQuery(searchParams.ActorID, searchParams.CustomerID, searchParams.AccountID)
	if actorIDQuery != "" {
		q = q.Where(actorIDQuery, actorIDArgs)
	}

	if searchParams.Name != "" {
		q = q.Where("name ILIKE ?", fmt.Sprintf("%%%s%%", searchParams.Name))
	}

	if searchParams.Description != "" {
		q = q.Where("description ILIKE ?", fmt.Sprintf("%%%s%%", searchParams.Description))
	}

	if searchParams.Environment != "" {
		q = q.Where("environment = ?", searchParams.Environment)
	}

	if searchParams.ExpiresAt != nil {
		q = q.Where("expires_at = ?", searchParams.ExpiresAt.GormValue())
	}

	if searchParams.ExpiresBefore != nil {
		q = q.Where("expires_at < ?", searchParams.ExpiresBefore.GormValue())
	}

	if searchParams.ExpiresAfter != nil {
		q = q.Where(s.client.Or("expires_at > ?", searchParams.ExpiresAfter.GormValue()).Or("expires_at IS NULL"))
	}

	if searchParams.Active != nil {
		q = q.Where("active = ?", *searchParams.Active)
	}

	if searchParams.Deleted != nil {
		if *searchParams.Deleted {
			q = q.Where("deleted_at IS NOT NULL")
		} else {
			q = q.Where("deleted_at IS NULL")
		}
	}

	return q
}
