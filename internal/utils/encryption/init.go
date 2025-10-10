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
	"errors"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"io"
)

// generatePassword generates a salted PBKDF2 hash of a password
func GeneratePassword(pass string) (string, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", err
	}
	salt := hex.EncodeToString(saltBytes)

	hashBytes := pbkdf2.Key([]byte(pass), saltBytes, 100000, 64, sha512.New)
	hash := hex.EncodeToString(hashBytes)

	return fmt.Sprintf("%s:%s", salt, hash), nil
}

// validatePassword verifies a password against a stored hash
func ValidatePassword(storedPassword, providedPassword string) (bool, error) {
	parts := [2]string{}
	split := 0
	for i, c := range storedPassword {
		if c == ':' {
			parts[0] = storedPassword[:i]
			parts[1] = storedPassword[i+1:]
			split = 1
			break
		}
	}
	if split == 0 {
		return false, errors.New("invalid stored password format")
	}

	salt, storedHash := parts[0], parts[1]

	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return false, err
	}

	storedHashBytes, err := hex.DecodeString(storedHash)
	if err != nil {
		return false, err
	}

	hashBytes := pbkdf2.Key([]byte(providedPassword), saltBytes, 100000, 64, sha512.New)

	// Timing-safe comparison
	if subtle.ConstantTimeCompare(hashBytes, storedHashBytes) == 1 {
		return true, nil
	}
	return false, nil
}

// PKCS7Padding adds padding to the plaintext
func pKCS7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

// PKCS7Unpadding removes padding
func pKCS7Unpadding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("invalid data")
	}
	padding := int(data[length-1])
	if padding > length {
		return nil, fmt.Errorf("invalid padding")
	}
	return data[:length-padding], nil
}

// EncryptAES encrypts plaintext using AES-CBC with PKCS7 padding
func EncryptAES(plaintext, key string) (string, error) {
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		// pad key to 32 bytes
		keyBytes = append(keyBytes, bytes.Repeat([]byte("0"), 32-len(keyBytes))...)
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	// generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	padded := pKCS7Padding([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// prepend IV to ciphertext (CryptoJS also does this internally in some formats)
	final := append(iv, ciphertext...)

	return base64.StdEncoding.EncodeToString(final), nil
}

// DecryptAES decrypts the base64 ciphertext
func DecryptAES(cipherTextBase64, key string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherTextBase64)
	if err != nil {
		return "", err
	}

	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		keyBytes = append(keyBytes, bytes.Repeat([]byte("0"), 32-len(keyBytes))...)
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	plaintext, err := pKCS7Unpadding(ciphertext)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
