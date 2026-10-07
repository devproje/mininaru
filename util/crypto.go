// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

const secretKeyFile = "secret.key"
const credentialsDBFile = "credentials.db"
const secretKeyBytes = 32
const encPrefix = "enc:v1:"

var secretKeyMu sync.Mutex

func generateSecretKey() (string, error) {
	var raw []byte

	var err error

	raw = make([]byte, secretKeyBytes)

	_, err = rand.Read(raw)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}

func decodeSecretKey(value string) ([]byte, error) {
	var key []byte

	var err error

	key, err = hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	if len(key) != secretKeyBytes {
		return nil, fmt.Errorf("invalid encryption key length")
	}

	return key, nil
}

func openCredentialsDB() (*sql.DB, error) {
	var path string
	var db *sql.DB

	var err error

	path = Path(credentialsDBFile)
	db, err = sql.Open("sqlite", databaseDSN(path))
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS credentials (name TEXT PRIMARY KEY, value TEXT NOT NULL);")
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	err = os.Chmod(path, 0600)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func removeLegacySecretKey(key []byte) error {
	var data []byte
	var legacyKey []byte

	var err error

	data, err = os.ReadFile(Path(secretKeyFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	legacyKey, err = decodeSecretKey(string(data))
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(legacyKey, key) != 1 {
		return fmt.Errorf("legacy encryption key differs from credentials database")
	}

	err = os.Remove(Path(secretKeyFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func secretKey(create bool) ([]byte, error) {
	var db *sql.DB
	var data []byte
	var key string
	var decodedKey []byte

	var err error

	secretKeyMu.Lock()
	defer secretKeyMu.Unlock()

	db, err = openCredentialsDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	err = db.QueryRow("SELECT value FROM credentials WHERE name = 'master';").Scan(&key)
	if err == nil {
		decodedKey, err = decodeSecretKey(key)
		if err != nil {
			return nil, err
		}
		err = removeLegacySecretKey(decodedKey)
		if err != nil {
			return nil, err
		}
		return decodedKey, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	data, err = os.ReadFile(Path(secretKeyFile))
	if err == nil {
		key = strings.TrimSpace(string(data))
		_, err = decodeSecretKey(key)
		if err != nil {
			return nil, err
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if os.IsNotExist(err) {
		if !create {
			return nil, fmt.Errorf("encryption key is missing")
		}
		key, err = generateSecretKey()
		if err != nil {
			return nil, err
		}
	}

	_, err = db.Exec("INSERT OR IGNORE INTO credentials (name, value) VALUES ('master', ?);", key)
	if err != nil {
		return nil, err
	}

	err = db.QueryRow("SELECT value FROM credentials WHERE name = 'master';").Scan(&key)
	if err != nil {
		return nil, err
	}
	decodedKey, err = decodeSecretKey(key)
	if err != nil {
		return nil, err
	}
	err = removeLegacySecretKey(decodedKey)
	if err != nil {
		return nil, err
	}
	return decodedKey, nil
}

func Encrypt(plaintext string) (string, error) {
	var key []byte
	var block cipher.Block
	var gcm cipher.AEAD
	var nonce []byte
	var sealed []byte

	var err error

	if plaintext == "" {
		return "", nil
	}

	key, err = secretKey(true)
	if err != nil {
		return "", err
	}

	block, err = aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err = cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce = make([]byte, gcm.NonceSize())

	_, err = rand.Read(nonce)
	if err != nil {
		return "", err
	}

	sealed = gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return encPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func Decrypt(value string) (string, error) {
	var key []byte
	var raw []byte
	var block cipher.Block
	var gcm cipher.AEAD
	var nonceSize int
	var plain []byte

	var err error

	if value == "" {
		return "", nil
	}

	if !strings.HasPrefix(value, encPrefix) {
		return value, nil
	}

	key, err = secretKey(false)
	if err != nil {
		return "", err
	}

	raw, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encPrefix))
	if err != nil {
		return "", err
	}

	block, err = aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err = cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize = gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("encrypted value is too short")
	}

	plain, err = gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}
