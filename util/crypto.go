// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

const secretKeyFile = "secret.key"
const secretKeyBytes = 32
const encPrefix = "enc:v1:"

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

func secretKey() ([]byte, error) {
	var path string
	var data []byte
	var key string

	var err error

	path = Path(secretKeyFile)

	data, err = os.ReadFile(path)
	if err == nil {
		return hex.DecodeString(strings.TrimSpace(string(data)))
	}

	if !os.IsNotExist(err) {
		return nil, err
	}

	key, err = generateSecretKey()
	if err != nil {
		return nil, err
	}

	err = WriteFileAtomic(path, []byte(key+"\n"), 0600)
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(key)
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

	key, err = secretKey()
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

	key, err = secretKey()
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
