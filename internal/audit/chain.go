package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Chain struct {
	mu       sync.Mutex
	prevHash string
	sidecar  string
}

func NewChain(dir string) *Chain {
	c := &Chain{sidecar: filepath.Join(dir, ".chain")}
	data, err := os.ReadFile(c.sidecar)
	if err == nil && len(data) == 64 {
		c.prevHash = string(data)
	}
	return c
}

func (c *Chain) Seal(eventJSON []byte) (prevHash, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prevHash = c.prevHash
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(eventJSON)
	hash = hex.EncodeToString(h.Sum(nil))
	c.prevHash = hash
	_ = os.WriteFile(c.sidecar, []byte(hash), 0o600)
	return prevHash, hash
}

func (c *Chain) PrevHash() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.prevHash
}

func VerifyFile(path string) (valid bool, brokenAt int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return false, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	prevHash := ""
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		var evt struct {
			PrevHash string `json:"prev_hash"`
			Hash     string `json:"hash"`
		}
		if err := json.Unmarshal(line, &evt); err != nil {
			return false, lineNum, fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}

		if evt.PrevHash != prevHash {
			return false, lineNum, fmt.Errorf("line %d: prev_hash mismatch", lineNum)
		}

		body := stripHashFields(line)
		h := sha256.New()
		h.Write([]byte(prevHash))
		h.Write(body)
		expected := hex.EncodeToString(h.Sum(nil))

		if evt.Hash != expected {
			return false, lineNum, fmt.Errorf("line %d: hash mismatch", lineNum)
		}

		prevHash = evt.Hash
	}
	if err := scanner.Err(); err != nil {
		return false, lineNum, err
	}
	return true, 0, nil
}

func stripHashFields(line []byte) []byte {
	var evt Event
	if err := json.Unmarshal(line, &evt); err != nil {
		return line
	}
	evt.PrevHash = ""
	evt.Hash = ""
	out, err := json.Marshal(evt)
	if err != nil {
		return line
	}
	return out
}

func ComputeHash(prevHash string, eventJSON []byte) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(eventJSON)
	return hex.EncodeToString(h.Sum(nil))
}
