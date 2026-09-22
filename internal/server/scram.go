package server

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// scramIterations is what Postgres itself uses for a fresh verifier.
const scramIterations = 4096

// scramVerifier builds the SCRAM-SHA-256 verifier Postgres stores for a
// password, in the form ALTER ROLE ... PASSWORD accepts verbatim:
// SCRAM-SHA-256$<iterations>:<salt>$<StoredKey>:<ServerKey>, all base64.
//
// No SASLprep: the passwords here are generated or chosen by one operator,
// and Postgres also stores a password it cannot normalise as is.
func scramVerifier(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	salted, err := pbkdf2.Key(sha256.New, password, salt, scramIterations, sha256.Size)
	if err != nil {
		return "", fmt.Errorf("deriving key: %w", err)
	}
	clientKey := hmacSHA256(salted, "Client Key")
	storedKey := sha256.Sum256(clientKey)
	serverKey := hmacSHA256(salted, "Server Key")
	enc := base64.StdEncoding.EncodeToString
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s",
		scramIterations, enc(salt), enc(storedKey[:]), enc(serverKey)), nil
}

func hmacSHA256(key []byte, message string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return mac.Sum(nil)
}
