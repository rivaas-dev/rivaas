package policies

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http/httptest"
	"testing"
)

func TestGetPolicies(t *testing.T) {
	a := assert.New(t)
	a.Nil(nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	policies := []string{"yes", "well hello"}
	handler := NewHandler(policies)
	handler.GetPolicy(c)
	a.Equal(200, w.Code)
	result, err := ioutil.ReadAll(w.Result().Body)
	a.Nil(err)
	var resultPolicies []string
	err = json.Unmarshal(result, &resultPolicies)
	a.Nil(err)
	a.Equal(policies, resultPolicies)
}
