package crypto

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"filippo.io/age"
)

// GenerateKey creates a new age X25519 keypair
func GenerateKey() (*age.X25519Identity, error) {
	return age.GenerateX25519Identity()
}

// SaveKey writes the identity to a file with secure permissions
func SaveKey(identity *age.X25519Identity, path string) error {
	content := fmt.Sprintf("# created: %s\n# public key: %s\n%s\n",
		"", // age-keygen includes timestamp, we skip it for simplicity
		identity.Recipient().String(),
		identity.String(),
	)
	return os.WriteFile(path, []byte(content), 0600)
}

// LoadKey reads an age identity from a file
func LoadKey(path string) (*age.X25519Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseKey(string(data))
}

// ParseKey extracts the age identity from key file content
func ParseKey(content string) (*age.X25519Identity, error) {
	// Find the AGE-SECRET-KEY line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
			return age.ParseX25519Identity(line)
		}
	}
	return nil, fmt.Errorf("no AGE-SECRET-KEY found in content")
}

// GetPublicKey extracts the public key from a key file
func GetPublicKey(path string) (string, error) {
	identity, err := LoadKey(path)
	if err != nil {
		return "", err
	}
	return identity.Recipient().String(), nil
}

// GetPublicKeyFromContent extracts public key from key content
func GetPublicKeyFromContent(content string) (string, error) {
	// Try to find public key comment
	re := regexp.MustCompile(`# public key: (age1[a-z0-9]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// Otherwise parse the secret key and derive public key
	identity, err := ParseKey(content)
	if err != nil {
		return "", err
	}
	return identity.Recipient().String(), nil
}

// Encrypt encrypts data with the given public key
func Encrypt(publicKey string, plaintext []byte) ([]byte, error) {
	recipient, err := age.ParseX25519Recipient(publicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	buf := &bytes.Buffer{}
	w, err := age.Encrypt(buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close encryptor: %w", err)
	}

	return buf.Bytes(), nil
}

// Decrypt decrypts data with the given identity
func Decrypt(identity *age.X25519Identity, ciphertext []byte) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return io.ReadAll(r)
}

// EncryptFile encrypts a file and writes to destination
func EncryptFile(publicKey, srcPath, dstPath string) error {
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	ciphertext, err := Encrypt(publicKey, plaintext)
	if err != nil {
		return err
	}

	return os.WriteFile(dstPath, ciphertext, 0644)
}

// DecryptFile decrypts a file and writes to destination
func DecryptFile(identity *age.X25519Identity, srcPath, dstPath string) error {
	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	plaintext, err := Decrypt(identity, ciphertext)
	if err != nil {
		return err
	}

	return os.WriteFile(dstPath, plaintext, 0644)
}

// ValidateKeyContent checks if content contains a valid age key
func ValidateKeyContent(content string) error {
	_, err := ParseKey(content)
	return err
}
