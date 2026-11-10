package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/logging"
)

// Box 用本机密钥加密连接口令。密钥不写入日志。
type Box struct {
	key []byte
}

func Load(cfg *config.Config, log *logging.Logger) (*Box, error) {
	var raw []byte
	switch {
	case cfg.Auth.Secret != "":
		sum := sha256.Sum256([]byte(cfg.Auth.Secret))
		raw = sum[:]
	default:
		path := filepath.Join(cfg.App.DataDir, "auth.key")
		existing, err := os.ReadFile(path)
		if err == nil && len(existing) == 32 {
			raw = existing
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("读取本地密钥失败: %w", err)
		}
		raw = make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return nil, fmt.Errorf("生成本地密钥失败: %w", err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			return nil, fmt.Errorf("保存本地密钥失败: %w", err)
		}
		log.Info("已生成连接口令密钥", "path", path)
	}
	return &Box{key: raw}, nil
}

func (b *Box) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (b *Box) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("口令密文无效")
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", fmt.Errorf("口令密文无效")
	}
	nonce := sealed[:gcm.NonceSize()]
	plain, err := gcm.Open(nil, nonce, sealed[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("口令密文无法解密")
	}
	return string(plain), nil
}
