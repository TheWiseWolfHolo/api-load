package utils

import (
	"crypto/sha256"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/pbkdf2"
)

var aesKeyDerivationSalt = []byte{
	0x67, 0x70, 0x74, 0x2d, 0x6c, 0x6f, 0x61, 0x64,
	0x2d, 0x65, 0x6e, 0x63, 0x72, 0x79, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x2d, 0x76, 0x31,
}

// ValidatePasswordStrength validates password strength with fixed minimum length of 16 characters
func ValidatePasswordStrength(password, fieldName string) {
	if len(password) < 16 {
		logrus.Warnf("%s is shorter than 16 characters, consider using a longer password", fieldName)
	}

	lower := strings.ToLower(password)
	weakPatterns := []string{"password", "sk-123456", "123456", "admin", "secret"}

	for _, pattern := range weakPatterns {
		if strings.Contains(lower, pattern) {
			logrus.Warnf("%s contains common weak patterns, consider using a stronger password", fieldName)
			break
		}
	}
}

// DeriveAESKey derives a 32-byte AES key from password using PBKDF2
func DeriveAESKey(password string) []byte {
	return pbkdf2.Key([]byte(password), aesKeyDerivationSalt, 100000, 32, sha256.New)
}
