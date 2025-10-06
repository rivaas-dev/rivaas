package filters

import (
	"testing"
)

func TestNewSearchParameters_NoMatch_ReturnsEmpty(t *testing.T) {
	search := NewSearchParameters(FilterParam{Match: map[string]string{"unknown": "abc"}})

	if search.Name != nil {
		t.Fatalf("expected Name to be nil, got %v", *search.Name)
	}
}

func TestNewSearchParameters_EmptyName_ReturnsNil(t *testing.T) {
	search := NewSearchParameters(FilterParam{Match: map[string]string{"name": ""}})
	if search.Name != nil {
		t.Fatalf("expected Name to be nil when empty, got %v", *search.Name)
	}
}

func TestNewSearchParameters_WithName_SetsPointer(t *testing.T) {
	search := NewSearchParameters(FilterParam{Match: map[string]string{"name": "Acme"}})
	if search.Name == nil || *search.Name != "Acme" {
		t.Fatalf("expected Name=Acme, got %+v", search.Name)
	}
}
