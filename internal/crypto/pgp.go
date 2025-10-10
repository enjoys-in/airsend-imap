package pgp

import (
	"fmt"
	"github.com/ProtonMail/gopenpgp/v3/crypto"
)

func DecryptMessage(publicKey, privateKey, password, encryptedMessage string) []byte {

	passphrase := []byte(password) // Passphrase of the privKey
	_, err := crypto.NewKeyFromArmored(publicKey)
	if err != nil {
		fmt.Printf("Public key error: %v\n", err)

	}

	privateKeyObj, privateKeyErr := crypto.NewPrivateKeyFromArmored(privateKey, passphrase)
	if privateKeyErr != nil {
		fmt.Printf("Private key error: %v\n", privateKeyErr)
	}
	pgp := crypto.PGP()

	decHandle, err := pgp.Decryption().DecryptionKey(privateKeyObj).New()
	decrypted, err := decHandle.Decrypt([]byte(encryptedMessage), crypto.Armor)
	myMessage := decrypted.Bytes()

	decHandle.ClearPrivateParams()

	return myMessage
}
