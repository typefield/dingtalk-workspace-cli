// Copyright 2026 Alibaba Group
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

package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/security"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/config"
)

type secureTempFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

var (
	secureGetMAC        = security.GetMACAddress
	secureMkdirAll      = os.MkdirAll
	secureStat          = os.Stat
	secureChmod         = os.Chmod
	secureMarshalIndent = json.MarshalIndent
	secureEncrypt       = security.Encrypt
	secureOpenFile      = func(name string, flag int, perm os.FileMode) (secureTempFile, error) {
		return os.OpenFile(name, flag, perm)
	}
	secureRemove    = os.Remove
	secureRename    = os.Rename
	secureReadFile  = os.ReadFile
	secureDecrypt   = security.Decrypt
	secureUnmarshal = json.Unmarshal
)

const secureDataFile = ".data"

// ErrTokenDecryption indicates that token decryption failed, typically
// due to a device mismatch or corrupted data file. Callers can check
// this with errors.Is to distinguish decryption failures from other
// I/O or parsing errors.
var ErrTokenDecryption = errors.New("token decryption failed")

// cachedMAC stores the MAC address to avoid repeated lookups.
var (
	cachedMAC     string
	cachedMACOnce sync.Once
	cachedMACErr  error
)

// getCachedMAC returns the cached MAC address, fetching it once if needed.
func getCachedMAC() (string, error) {
	cachedMACOnce.Do(func() {
		cachedMAC, cachedMACErr = secureGetMAC()
	})
	return cachedMAC, cachedMACErr
}

// resolvePassword builds the encryption password from MAC address.
func resolvePassword() ([]byte, error) {
	mac, err := getCachedMAC()
	if err != nil {
		return nil, fmt.Errorf("getting MAC address for encryption: %w", err)
	}
	return []byte(mac), nil
}

// SaveSecureTokenData encrypts and saves TokenData to .data file.
// The data is encrypted using AES-256-GCM with a key derived from the device
// MAC address. Uses atomic write (write .tmp then rename) to prevent
// corruption.
//
// Concurrency: callers that involve token refresh MUST hold the business-level
// file lock (via acquireTokenLock) to prevent two processes from refreshing
// simultaneously. See OAuthProvider.lockedRefresh().
func SaveSecureTokenData(configDir string, data *TokenData) error {
	password, err := resolvePassword()
	if err != nil {
		return err
	}

	if err := secureMkdirAll(configDir, config.DirPerm); err != nil {
		return fmt.Errorf("creating config dir %s: %w", configDir, err)
	}
	// Verify the directory permissions are strict even if it already existed.
	if info, statErr := secureStat(configDir); statErr == nil {
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			if chErr := secureChmod(configDir, config.DirPerm); chErr != nil {
				return fmt.Errorf("config dir %s has unsafe permissions %o and chmod failed: %w", configDir, perm, chErr)
			}
		}
	}

	plaintext, err := secureMarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token data: %w", err)
	}
	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	ciphertext, err := secureEncrypt(plaintext, password)
	if err != nil {
		return fmt.Errorf("encrypting token data: %w", err)
	}

	finalPath := filepath.Join(configDir, secureDataFile)
	// Give each writer its own temp file. A fixed ".data.tmp" path lets a
	// failed concurrent writer remove another writer's in-flight temp file,
	// which can leave the final file missing on Windows.
	tmpPath := finalPath + "." + uuid.New().String() + ".tmp"

	// Atomic write with fsync to ensure data durability
	tmpFile, err := secureOpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, config.FilePerm)
	if err != nil {
		return fmt.Errorf("creating tmp file: %w", err)
	}

	writeSuccess := false
	defer func() {
		if !writeSuccess {
			tmpFile.Close()
			_ = secureRemove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(ciphertext); err != nil {
		return fmt.Errorf("writing tmp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("syncing tmp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing tmp file: %w", err)
	}
	if err := secureRename(tmpPath, finalPath); err != nil {
		_ = secureRemove(tmpPath)
		return fmt.Errorf("renaming tmp to final: %w", err)
	}
	writeSuccess = true

	return nil
}

// LoadSecureTokenData decrypts and loads TokenData from .data file.
// Reads are safe without locking because SaveSecureTokenData uses atomic rename.
func LoadSecureTokenData(configDir string) (*TokenData, error) {
	password, err := resolvePassword()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(configDir, secureDataFile)
	ciphertext, err := secureReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading secure data file: %w", err)
	}

	plaintext, err := secureDecrypt(ciphertext, password)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenDecryption, err)
	}
	// Zero sensitive plaintext as soon as unmarshalling completes.
	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	var data TokenData
	if err := secureUnmarshal(plaintext, &data); err != nil {
		return nil, fmt.Errorf("parsing decrypted token data: %w", err)
	}

	return &data, nil
}

// DeleteSecureData removes .data file from configDir.
func DeleteSecureData(configDir string) error {
	path := filepath.Join(configDir, secureDataFile)
	if err := secureRemove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting secure data file: %w", err)
	}
	_ = secureRemove(path + ".tmp")
	entries, _ := os.ReadDir(configDir)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, secureDataFile+".") || !strings.HasSuffix(name, ".tmp") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, secureDataFile+"."), ".tmp")
		if _, err := uuid.Parse(id); err == nil {
			_ = secureRemove(filepath.Join(configDir, name))
		}
	}
	return nil
}

// SecureDataExists checks if the secure .data file exists in the given directory.
func SecureDataExists(configDir string) bool {
	path := filepath.Join(configDir, secureDataFile)
	_, err := secureStat(path)
	return err == nil
}
