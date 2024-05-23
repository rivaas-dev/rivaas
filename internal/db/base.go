package db

// DatabaseExecer Key Repository interface
type DatabaseExecer interface {
	GetKey(hash string) (*Key, error)
	GetKeys(actorID, description, customerID, accountID string) ([]*Key, error)
}
