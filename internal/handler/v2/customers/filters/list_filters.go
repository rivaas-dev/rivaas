package filters

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
)

const (
	nameFilter = `name`
)

type FilterParam struct {
	Match map[string]string
}

// NewSearchParameters construct search parameters from maps struct
func NewSearchParameters(params FilterParam) customers.CustomerSearch {
	var search customers.CustomerSearch
	if name, ok := params.Match[nameFilter]; ok && name != "" {
		search.Name = &name
	}

	return search
}
