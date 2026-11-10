package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"

	"github.com/cng1985/dbgo/internal/platform/config"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

const bcryptCost = 12

var dummyHash []byte

func init() {
	hash, err := bcrypt.GenerateFromPassword([]byte("dbgo-dummy-password"), bcryptCost)
	if err != nil {
		panic(err)
	}
	dummyHash = hash
}

func hashPassword(password string) (string, error) {
	if len(password) < 8 || len(password) > 72 {
		return "", apperrors.Invalid("密码长度需要在 8 到 72 之间")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", apperrors.Internal(err)
	}
	return string(hash), nil
}

func checkPassword(hash, password string) bool {
	if hash == "" {
		hash = string(dummyHash)
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func newToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	out := make([]byte, len(raw))
	for i, value := range raw {
		out[i] = alphabet[int(value)%len(alphabet)]
	}
	return string(out), nil
}

func adminPasswordPath(cfg *config.Config) string {
	return filepath.Join(cfg.App.DataDir, "initial-admin-password.txt")
}

func writeAdminPassword(path, password string) error {
	if err := os.WriteFile(path, []byte(password+"\n"), 0o600); err != nil {
		return apperrors.Wrap(apperrors.KindInternal, 500, "无法保存初始管理员口令", err)
	}
	return nil
}
