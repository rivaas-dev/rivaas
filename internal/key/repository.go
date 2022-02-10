package key

//go:generate mockgen -destination=./mock_keys_repository.go -package=key -source=repository.go RepositoryInterface

//RepositoryInterface Key Repository
type RepositoryInterface interface {
	StoreKey(key Key) error
	GetKeyByHash(hash string) (*Key, error)
	GetKeysByActorID(actorID string) ([]*Key, error)
	ListKeys() ([]*Key, error)
}
