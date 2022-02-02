package policies

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

//Handler handles policy requests
type Handler struct {
	policies []string
}

//NewHandler constructor
func NewHandler(policies []string) *Handler {
	return &Handler{policies: policies}
}

//GetPolicy return the list of policies (the shortest handler ever)
func (h *Handler) GetPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, h.policies)
}
