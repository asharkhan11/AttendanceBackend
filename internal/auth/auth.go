package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"runtime"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/models"
	"golang.org/x/crypto/argon2"
)

// ─── Claims ───────────────────────────────────────────────────────────────────

type Claims struct {
	jwt.RegisteredClaims
	SchoolID uuid.UUID        `json:"school_id"`
	Role     models.UserRole  `json:"role"`
}

func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(models.CtxClaims).(*Claims)
	return c
}

func ContextWithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, models.CtxClaims, c)
}

// ─── JWT ──────────────────────────────────────────────────────────────────────

type JWTService struct {
	secret    []byte
	accessTTL time.Duration
}

func NewJWTService(secret string, accessTTL time.Duration) *JWTService {
	return &JWTService{secret: []byte(secret), accessTTL: accessTTL}
}

func (j *JWTService) IssueAccessToken(userID, schoolID uuid.UUID, role models.UserRole) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.accessTTL)),
		},
		SchoolID: schoolID,
		Role:     role,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(j.secret)
}

func (j *JWTService) VerifyAccessToken(tokenStr string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// ─── Password ─────────────────────────────────────────────────────────────────

type Argon2Params struct {
	MemoryKB    uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

var DefaultArgon2Params = Argon2Params{
	MemoryKB:    16384, // 16 MB RFC 9106 recommended minimal profile
	Iterations:  2,
	Parallelism: 1,
	SaltLen:     16,
	KeyLen:      32,
}

type PasswordService struct {
	params Argon2Params
	sem    chan struct{}
}

func NewPasswordService(memKB, iterations uint32, parallelism uint8) *PasswordService {
	p := DefaultArgon2Params
	if memKB > 0 {
		p.MemoryKB = memKB
	}
	if iterations > 0 {
		p.Iterations = iterations
	}
	if parallelism > 0 {
		p.Parallelism = parallelism
	}
	maxWorkers := runtime.NumCPU() * 2
	if maxWorkers < 2 {
		maxWorkers = 2
	}
	if maxWorkers > 4 {
		maxWorkers = 4
	}
	return &PasswordService{
		params: p,
		sem:    make(chan struct{}, maxWorkers),
	}
}

// HashPassword produces $argon2id$v=19$m=...,t=...,p=...$<b64salt>$<b64hash>
func (s *PasswordService) HashPassword(password string) (string, error) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	salt := make([]byte, s.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, s.params.Iterations, s.params.MemoryKB, s.params.Parallelism, s.params.KeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		s.params.MemoryKB, s.params.Iterations, s.params.Parallelism, b64Salt, b64Hash), nil
}

// VerifyPassword returns true if password matches the stored hash string.
func (s *PasswordService) VerifyPassword(password, encodedHash string) (bool, error) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	var version int
	var mem, iters uint32
	var para uint8
	var b64Salt, b64Hash string
	_, err := fmt.Sscanf(encodedHash,
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s",
		&version, &mem, &iters, &para, &b64Salt)
	if err != nil {
		return false, fmt.Errorf("malformed hash: %w", err)
	}
	// b64Salt contains "$<hash>" at the end since Sscanf reads until space
	// We need to split on "$"
	parts := splitLast(b64Salt, "$")
	if len(parts) != 2 {
		return false, fmt.Errorf("malformed hash segments")
	}
	b64Salt = parts[0]
	b64Hash = parts[1]

	salt, err := base64.RawStdEncoding.DecodeString(b64Salt)
	if err != nil {
		return false, fmt.Errorf("decoding salt: %w", err)
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(b64Hash)
	if err != nil {
		return false, fmt.Errorf("decoding hash: %w", err)
	}
	computed := argon2.IDKey([]byte(password), salt, iters, mem, para, uint32(len(expectedHash)))
	return constTimeEqual(computed, expectedHash), nil
}

func splitLast(s, sep string) []string {
	idx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if string(s[i]) == sep {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

// constTimeEqual compares two byte slices in constant time.
func constTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// ─── Refresh Tokens ───────────────────────────────────────────────────────────

// GenerateRefreshToken generates a cryptographically random token and returns
// both the raw token (to send to client) and the hashed version (to store in DB).
func GenerateRefreshToken() (raw string, hashed string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating refresh token: %w", err)
	}
	raw = base64.URLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hashed = hex.EncodeToString(h[:])
	return raw, hashed, nil
}

// HashRefreshToken returns the sha256 hex of a raw token (for lookup).
func HashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

const passCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

// GenerateRandomPassword creates a cryptographically secure random string of given length.
func GenerateRandomPassword(length int) (string, error) {
	if length <= 0 {
		length = 16
	}
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random password: %w", err)
	}
	for i, b := range bytes {
		bytes[i] = passCharset[int(b)%len(passCharset)]
	}
	return string(bytes), nil
}
