package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDoc(t *testing.T) {
	// Scan source data as HUML.
	var resHuml map[string]any
	b, err := os.ReadFile("tests/documents/mixed.huml")
	if err != nil {
		t.Fatalf("failed to read tests/documents/mixed.huml: %v", err)
	}
	if err := Unmarshal(b, &resHuml); err != nil {
		t.Fatalf("failed to unmarshal mixed.huml: %v", err)
	}

	// Marshal it back to HUML.
	marshalled, err := Marshal(resHuml)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	// Read it again.
	var resHumlConverted map[string]any
	if err := Unmarshal(marshalled, &resHumlConverted); err != nil {
		t.Fatalf("failed to unmarshal converted HUML: %v", err)
	}
	out := normalizeToJSON(resHumlConverted)

	// Read test.json and unmarshal it.
	var resJson map[string]any
	b, err = os.ReadFile("tests/documents/mixed.json")
	if err != nil {
		t.Fatalf("failed to read tests/documents/mixed.json: %v", err)
	}
	if err := json.Unmarshal(b, &resJson); err != nil {
		t.Fatalf("failed to unmarshal tests/documents/mixed.json: %v", err)
	}

	// Deep-compare both.
	assert.Equal(t, out, resJson, "test.huml and tests/documents/mixed.json should be deeply equal")
}
