package handler

import (
	"fmt"
	"strings"
)

const platformCI = "company.info"
const platformDS = "data-science"
const platformMI = "marketing-intelligence"
const platformWS = "webservices"

const namespaceCI = "ci"
const namespaceDS = "ds"
const namespaceMI = "mi"
const namespaceWS = "ws"

func resolvePlatform(urn string) (*string, error) {
	var p string
	ns := strings.ToLower(urn)[0:2]
	switch ns {
	case namespaceCI:
		p = platformCI
	case namespaceDS:
		p = platformDS
	case namespaceMI:
		p = platformMI
	case namespaceWS:
		p = platformWS
	default:
		return nil, fmt.Errorf("unrecognized namespace for URN `%s`", urn)
	}
	return &p, nil
}
