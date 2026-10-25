package storage

import (
	"path/filepath"
	"testing"
)

type TestPayload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestWriteJSONAtomic(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "sub", "data.json")

	payload := TestPayload{Name: "Ringmaster", Count: 42}
	if err := WriteJSONAtomic(targetPath, payload); err != nil {
		t.Fatalf("WriteJSONAtomic failed: %v", err)
	}

	read, err := ReadJSON[TestPayload](targetPath)
	if err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if read.Name != "Ringmaster" || read.Count != 42 {
		t.Errorf("read payload mismatch: %+v", read)
	}
}
