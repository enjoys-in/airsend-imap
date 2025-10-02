package interfaces

type CryptoPlugin interface {
	Encrypt(msg []byte, recipients []string) ([]byte, error)
	Decrypt(msg []byte, userID string) ([]byte, error)
}
