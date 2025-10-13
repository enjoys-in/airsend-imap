package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"io"
)

// PKCS7 padding
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// PKCS7 unpadding
func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("invalid padding size")
	}
	padding := int(data[length-1])
	if padding > length {
		return nil, fmt.Errorf("invalid padding length")
	}
	return data[:length-padding], nil
}

// AES-CBC Encryption
func EncryptAES(plaintext, key []byte) (string, string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}

	// Random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", "", err
	}

	// Pad plaintext
	padded := pkcs7Pad(plaintext, aes.BlockSize)

	// Encrypt
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Base64 encode for readability
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(iv), nil
}

// AES-CBC Decryption
func DecryptAES(cipherTextB64, ivB64 string, key []byte) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(cipherTextB64)
	if err != nil {
		return "", err
	}
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(cipherText)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(cipherText))
	mode.CryptBlocks(plaintext, cipherText)

	unpadded, err := pkcs7Unpad(plaintext)
	if err != nil {
		return "", err
	}

	return string(unpadded), nil
}

// ValidateSMTPPassword checks if a provided password matches a stored password using
// a salted hash using PBKDF2. It takes two parameters: the stored password
// and the provided password. It returns true if the provided password matches the
// stored password, and false otherwise. The stored password is expected to be in
// the format "salt:hash", where the hash is a hexadecimal string.

func ValidatePassword(storedPassword, providedPassword string) bool {
	parts := strings.Split(storedPassword, ":")
	if len(parts) != 2 {
		return false
	}
	salt, storedHash := parts[0], parts[1]
	derived := pbkdf2.Key([]byte(providedPassword), []byte(salt), 100000, 64, sha512.New)
	storedBytes, _ := hex.DecodeString(storedHash)
	return subtle.ConstantTimeCompare(derived, storedBytes) == 1
}
