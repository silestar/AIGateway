package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CryptoService AES-256-GCM 加解密服务
type CryptoService struct {
	gcm cipher.AEAD
}

// NewCrypto 创建加密服务
// secretKey 必须为 Base64 编码的 32 字节密钥
func NewCrypto(secretKey string) (*CryptoService, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(secretKey)
	if err != nil {
		return nil, fmt.Errorf("decode secret key: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("secret key must be 32 bytes, got %d", len(keyBytes))
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &CryptoService{gcm: gcm}, nil
}

// Encrypt 加密明文，返回 Base64 编码的密文
// 每次加密使用随机 Nonce，相同明文产生不同密文
func (c *CryptoService) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// nonce 放在密文前面，解密时需要取出
	ciphertext := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 Base64 编码的密文
func (c *CryptoService) Decrypt(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	nonceSize := c.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EnsureSecretKey 确保 SECRET_KEY 存在，不存在则自动生成
// 返回可用的密钥（Base64 编码）
func EnsureSecretKey(envPath string) (string, error) {
	// 先检查环境变量
	if key := os.Getenv("SECRET_KEY"); key != "" {
		return key, nil
	}

	// 检查 .env 文件
	data, err := os.ReadFile(envPath)
	if err == nil {
		for _, line := range splitLines(string(data)) {
			if len(line) > 11 && line[:11] == "SECRET_KEY=" {
				key := line[11:]
				if key != "" {
					os.Setenv("SECRET_KEY", key)
					return key, nil
				}
			}
		}
	}

	// 生成新密钥
	key := generateSecretKey()

	// 写入 .env
	if err := appendToEnvFile(envPath, "SECRET_KEY", key); err != nil {
		return "", fmt.Errorf("write secret key to .env: %w", err)
	}

	os.Setenv("SECRET_KEY", key)
	return key, nil
}

func generateSecretKey() string {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		// 极端情况下 rand 不可用，用 fallback
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(key)
}

func appendToEnvFile(path, key, value string) error {
	// 确保 .env 所在目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "\n%s=%s\n", key, value)
	return err
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
