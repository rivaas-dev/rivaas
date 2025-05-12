package db

// DatabaseExecer Key Repository interface
type DatabaseExecer interface {
	GetKey(hash string) (*Key, error)
	GetKeys(actorID, description, customerID, accountID string) ([]*Key, error)
	GetKeysPaginated(filterParams SearchParams, pageSize uint16, pageNumber uint32) (keys []*Key, totalResults int64, err error)
}
