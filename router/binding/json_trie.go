package binding

import (
	"encoding/json"
	"reflect"
)

// jsonFieldTrie represents allowed JSON field paths for unknown field detection.
type jsonFieldTrie struct {
	children map[string]*jsonFieldTrie
	isLeaf   bool // true if this is a valid terminal field
}

// newJSONFieldTrie builds a trie of allowed field paths from struct info.
func newJSONFieldTrie(t reflect.Type, tag string) *jsonFieldTrie {
	root := &jsonFieldTrie{children: make(map[string]*jsonFieldTrie)}
	buildTrie(root, t, tag, "")
	return root
}

// buildTrie recursively populates the trie with allowed field paths.
func buildTrie(node *jsonFieldTrie, t reflect.Type, tag string, path string) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	info := getStructInfo(t, tag)

	for _, field := range info.fields {
		fieldName := field.tagName
		if fieldName == "" || fieldName == "-" {
			continue
		}

		// Create child node
		if node.children[fieldName] == nil {
			node.children[fieldName] = &jsonFieldTrie{
				children: make(map[string]*jsonFieldTrie),
			}
		}
		child := node.children[fieldName]

		// If it's a nested struct, recurse
		if field.isStruct {
			buildTrie(child, field.fieldType, tag, path+fieldName+".")
		} else {
			// Terminal field (leaf)
			child.isLeaf = true
		}
	}
}

// walkJSONRawMessage walks JSON data from json.RawMessage and reports unknown fields.
// This is more efficient than unmarshaling to map[string]any first.
func walkJSONRawMessage(data json.RawMessage, trie *jsonFieldTrie, path []string, onUnknown func(string)) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	for key, value := range m {
		currentPath := append(path, key)
		pathStr := joinPath(currentPath)

		// Check if this field is allowed
		childTrie := trie.children[key]
		if childTrie == nil {
			// Unknown field detected
			onUnknown(pathStr)
			continue
		}

		// Try to determine if this is a nested object or array
		// Peek at first character to avoid full unmarshal
		trimmed := trimSpace(value)
		if len(trimmed) == 0 {
			continue
		}

		firstChar := trimmed[0]
		if firstChar == '{' {
			// Nested object - recurse
			walkJSONRawMessage(value, childTrie, currentPath, onUnknown)
		} else if firstChar == '[' {
			// Array - check if it's an array of objects
			var arr []json.RawMessage
			if err := json.Unmarshal(value, &arr); err == nil {
				for _, elem := range arr {
					// Check if element is an object
					elemTrimmed := trimSpace(elem)
					if len(elemTrimmed) > 0 && elemTrimmed[0] == '{' {
						walkJSONRawMessage(elem, childTrie, currentPath, onUnknown)
					}
				}
			}
		}
	}

	return nil
}

// trimSpace removes leading/trailing whitespace from JSON bytes.
func trimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\t' || b[start] == '\n' || b[start] == '\r') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\n' || b[end-1] == '\r') {
		end--
	}
	return b[start:end]
}

// joinPath creates a dot-separated path string.
func joinPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	result := path[0]
	for i := 1; i < len(path); i++ {
		result += "." + path[i]
	}
	return result
}
