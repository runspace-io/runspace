package resourceplugin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

func sealCredential(key []byte, value string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(value), nil), nil
}

func openCredential(key, sealed []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	size := gcm.NonceSize()
	if len(sealed) < size {
		return "", ErrInvalid
	}
	value, err := gcm.Open(nil, sealed[:size], sealed[size:], nil)
	return string(value), err
}
