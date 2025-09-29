// Package main demonstrates the test helper for testing logging functionality.
package main

import (
	"fmt"

	"rivaas.dev/logging"
)

// This example demonstrates using the test helper utilities without external packages.
func main() {
	// Create a JSON logger backed by an in-memory buffer
	l, buf := logging.NewTestLogger()

	l.Info("hello", "k", "v")
	l.Error("oops", "code", 500)

	entries, err := logging.ParseJSONLogEntries(buf)
	if err != nil {
		panic(err)
	}

	fmt.Printf("entries=%d first=%q last=%q\n", len(entries), entries[0].Message, entries[len(entries)-1].Message)
}
