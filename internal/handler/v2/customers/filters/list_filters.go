package filters

const (
	nameFilter = `name`
)

type FilterParam struct {
	Match map[string]string
}

type CustomerSearch struct {
	Name *string
	ID   *string
}

// NewSearchParameters construct search parameters from maps struct
func NewSearchParameters(params FilterParam) CustomerSearch {
	var search CustomerSearch
	if name, ok := params.Match[nameFilter]; ok && name != "" {
		search.Name = &name
	}

	return search
}
