// Package cryptoutil provides RSA password encryption for Steam authentication.
package cryptoutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

// EncryptPassword encrypts a password using the RSA public key returned by Steam.
//
// Steam's login flow requires:
//  1. Fetch RSA public key via GetPasswordRSAPublicKey (hex-encoded modulus + exponent)
//  2. Encrypt the password with PKCS1v15
//  3. Base64-encode the ciphertext
//  4. Submit the encrypted password + timestamp to BeginAuthSessionViaCredentials
//
// pubKeyMod and pubKeyExp are hex-encoded strings as returned by Steam's API.
func EncryptPassword(password, pubKeyMod, pubKeyExp string) (string, error) {
	modulus := new(big.Int)
	if _, ok := modulus.SetString(pubKeyMod, 16); !ok {
		return "", fmt.Errorf("cryptoutil: invalid RSA modulus hex string")
	}

	exponent := new(big.Int)
	if _, ok := exponent.SetString(pubKeyExp, 16); !ok {
		return "", fmt.Errorf("cryptoutil: invalid RSA exponent hex string")
	}

	pubKey := &rsa.PublicKey{
		N: modulus,
		E: int(exponent.Int64()),
	}

	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, []byte(password))
	if err != nil {
		return "", fmt.Errorf("cryptoutil: RSA encryption failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}
