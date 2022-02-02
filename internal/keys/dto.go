package keys

//PostKeyRequest request
type PostKeyRequest struct {
	Policies []string `json:"policies"`
	URN      string   `json:"urn" binding:"required"`
}

//PostKeyResponse response
type PostKeyResponse struct {
	ID   string `json:"key"`
	Hash string `json:"hash"`
}
