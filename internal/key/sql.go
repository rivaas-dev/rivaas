package key

import (
	"errors"
	"fmt"
	dsn "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

//SQLRepository sql implementation
type SQLRepository struct {
	client *gorm.DB
}

//NewSQLRepository constructor
func NewSQLRepository(client *gorm.DB) *SQLRepository {
	return &SQLRepository{client: client}
}

//NewSQLRepositoryFromCredentials constructor from db parameters
func NewSQLRepositoryFromCredentials(address string, username string, password string, database string) (*SQLRepository, error) {
	cfg := dsn.Config{
		User:                 username,
		Passwd:               password,
		DBName:               database,
		Addr:                 address,
		Net:                  "tcp",
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	d := cfg.FormatDSN()
	db, err := gorm.Open(mysql.Open(d), &gorm.Config{})
	return NewSQLRepository(db), err
}

//StoreKey does what it says
func (s SQLRepository) StoreKey(key Key) error {
	err := s.client.Create(&key).Error
	return err
}

//GetKeyByHash single key
func (s SQLRepository) GetKeyByHash(hash string) (*Key, error) {
	var k Key
	res := s.client.First(&k, "key_hash = ?", hash)
	if res.Error != nil && errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil // not really an error, just no result
	}
	return &k, res.Error
}

//GetKeys get all the keys with optional filters
func (s SQLRepository) GetKeys(input GetKeysInput) ([]*Key, error) {
	//q, args := buildGetKeysQueryAndArgsFromParameters(actorID, description)
	var keyList []*Key
	q := s.client
	searchKey := Key{ActorID: input.ActorID}
	q = q.Where(searchKey)
	if input.Description != nil {
		q = q.Where("description LIKE ?", fmt.Sprintf("%%%s%%", *input.Description))
	}
	result := q.Find(&keyList)
	if result.Error != nil {
		return nil, result.Error
	}

	return keyList, nil
}

//UpdateKeyByHash update a key
func (s SQLRepository) UpdateKeyByHash(hash string, values map[string]interface{}) (*Key, error) {
	var k Key
	res := s.client.First(&k, "key_hash = ?", hash)
	if res.Error != nil {
		return nil, res.Error
	}
	res = s.client.Model(&k).Updates(values)
	return &k, res.Error
}
