package key

import (
	"database/sql"
	"github.com/go-sql-driver/mysql"
)

const (
	StoreKeyQueryTemplate       = `INSERT INTO api_keys(key_hash, actor_id, expiration_date, description) VALUES (?, ?, ?, ?)`
	GetKeyByHashQueryTemplate   = "SELECT key_hash, actor_id, expiration_date FROM api_keys WHERE key_hash = ?"
	GetKeysByActorQueryTemplate = "SELECT key_hash, actor_id, expiration_date FROM api_keys WHERE actor_id = ?"
	ListKeysQueryTemplate       = "SELECT key_hash, actor_id, expiration_date FROM api_keys"
)

//SQLRepository sql implementation
type SQLRepository struct {
	client *sql.DB
}

//NewSQLRepository constructor
func NewSQLRepository(client *sql.DB) *SQLRepository {
	return &SQLRepository{client: client}
}

//NewSQLRepositoryFromCredentials constructor from db parameters
func NewSQLRepositoryFromCredentials(address string, username string, password string, database string) (*SQLRepository, error) {
	cfg := mysql.Config{
		User:                 username,
		Passwd:               password,
		DBName:               database,
		Addr:                 address,
		Net:                  "tcp",
		AllowNativePasswords: true,
	}

	var err error
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err

	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return NewSQLRepository(db), nil
}

//StoreKey does what it says
func (s SQLRepository) StoreKey(key Key) error {
	_, err := s.client.Exec(StoreKeyQueryTemplate, key.Hash, key.ActorID, key.ExpirationDate, key.Description)
	return err
}

// TODO do when we implement the /keys endpoint
func (s SQLRepository) GetKeyByHash(hash string) (*Key, error) {
	panic("implement me")
}

// TODO do when we implement the /keys endpoint
func (s SQLRepository) GetKeysByActorID(actorID string) ([]*Key, error) {
	panic("implement me")
}

// TODO do when we implement the /keys endpoint
func (s SQLRepository) ListKeys() ([]*Key, error) {
	panic("implement me")
}
