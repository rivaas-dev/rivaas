package key

//go:generate mockgen -destination=./mock_keys_repository.go -package=key -source=repository.go RepositoryInterface

//RepositoryInterface Key Repository
type RepositoryInterface interface {
	StoreKey(key Key) error
	GetKeyByHash(hash string) (*Key, error)
	GetKeys(input GetKeysInput) ([]*Key, error)
}

//GetKeysInput input
type GetKeysInput struct {
	ActorID     string  `form:"actor_id"`
	Description *string `form:"description"`
}
