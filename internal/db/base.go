package db

// DatabaseExecer Key Repository interface
type DatabaseExecer interface {
	GetKey(hash string) (*Key, error)
	GetKeys(actorID, description string) ([]*Key, error)
}
