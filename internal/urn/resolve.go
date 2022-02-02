package urn

import (
	"fmt"
	"strings"
)

// TODO move to centralized URN package

const PlatformCI = "company.info"
const PlatformDS = "data-science"
const PlatformMI = "marketing-intelligence"
const PlatformWS = "webservices"

const NamespaceCI = "ci"
const NamespaceDS = "ds"
const NamespaceMI = "mi"
const NamespaceWS = "ws"

func ResolvePlatform(urn string) (*string, error) {
	var p string
	ns := strings.ToLower(urn)[0:2]
	switch ns {
	case NamespaceCI:
		p = PlatformCI
	case NamespaceDS:
		p = PlatformDS
	case NamespaceMI:
		p = PlatformMI
	case NamespaceWS:
		p = PlatformWS
	default:
		return nil, fmt.Errorf("unrecognized namespace for URN `%s`", urn)
	}
	return &p, nil
}
