package keys

//RepositoryInterface Key Repository
type RepositoryInterface interface {
	StoreKey(key Key) error
	GetKeyByHash(hash string) (*Key, error)
	GetKeysByActorID(actorID string) ([]*Key, error)
	ListKeys() ([]*Key, error)
}
