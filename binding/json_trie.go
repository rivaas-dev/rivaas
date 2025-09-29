// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package binding

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
)

// jsonFieldTrie represents allowed JSON field paths for unknown field detection.
// It is a trie data structure that maps field names to child nodes, enabling
// path lookups for nested structures.
type jsonFieldTrie struct {
	children map[string]*jsonFieldTrie
	isLeaf   bool // true if this is a valid terminal field
}

// newJSONFieldTrie builds a trie of allowed field paths from struct type information.
// It recursively processes struct fields to build the complete path tree.
func newJSONFieldTrie(t reflect.Type, tag string) *jsonFieldTrie {
	root := &jsonFieldTrie{children: make(map[string]*jsonFieldTrie)}
	buildTrie(root, t, tag, "")
	return root
}

// buildTrie recursively populates the trie with allowed field paths.
// It processes struct fields and marks nested structs for recursive processing,
// while marking terminal fields as leaves.
func buildTrie(node *jsonFieldTrie, t reflect.Type, tag string, path string) {
	if t.Kind() == reflect.Pointer {
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
// It processes the raw JSON without fully unmarshaling, checking each field path
// against the trie of allowed fields.
func walkJSONRawMessage(data json.RawMessage, trie *jsonFieldTrie, path []string, onUnknown func(string)) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	for key, value := range m {
		currentPath := append(path, key)
		pathStr := strings.Join(currentPath, ".")

		childTrie := trie.children[key]
		if childTrie == nil {
			onUnknown(pathStr)
			continue
		}

		if err := walkValue(value, childTrie, currentPath, onUnknown); err != nil {
			return err
		}
	}

	return nil
}

// walkValue processes a JSON value and recurses if it is an object or array.
// Primitive values are ignored as they cannot contain unknown fields.
func walkValue(value json.RawMessage, trie *jsonFieldTrie, path []string, onUnknown func(string)) error {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return nil
	}

	switch trimmed[0] {
	case '{':
		// Nested object - recurse
		return walkJSONRawMessage(value, trie, path, onUnknown)
	case '[':
		// Array - check if it's an array of objects
		return walkArray(value, trie, path, onUnknown)
	}
	return nil
}

// walkArray processes a JSON array and walks each object element.
// It only processes array elements that are objects, skipping primitives.
func walkArray(data json.RawMessage, trie *jsonFieldTrie, path []string, onUnknown func(string)) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil // Not an array or invalid JSON - ignore
	}

	for _, elem := range arr {
		elemTrimmed := bytes.TrimSpace(elem)
		if len(elemTrimmed) > 0 && elemTrimmed[0] == '{' {
			if err := walkJSONRawMessage(elem, trie, path, onUnknown); err != nil {
				return err
			}
		}
	}
	return nil
}
