package db

// DatabaseExecer Key Repository interface
type DatabaseExecer interface {
	GetKey(hash string) (*Key, error)
	GetKeys(actorID, description, customerID, accountID string) ([]*Key, error)
	GetKeysPaginated(filterParams SearchParams, pageSize uint, pageNumber uint) (keys []*Key, totalResults int64, err error)
	GetKeysCountPerEnvironment(searchParams SearchParams) (totalResults map[Environment]int64, err error)
}
