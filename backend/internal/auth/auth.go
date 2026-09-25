package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var ErrUnauthorized = errors.New("unauthorized")

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"-"`
}

func (User) TableName() string { return "admin_users" }

type Session struct {
	TokenHash string `gorm:"primaryKey"`
	AdminID   string
	ExpiresAt time.Time
	CreatedAt time.Time
}
type Service struct {
	DB  *gorm.DB
	TTL time.Duration
}

func Token() string { var b [32]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func CSRF(token string) string { return Hash("csrf:" + token) }
func CheckCSRF(token, provided string) bool {
	return subtle.ConstantTimeCompare([]byte(CSRF(token)), []byte(provided)) == 1
}

func CreateUser(ctx context.Context, db *gorm.DB, username, password string) error {
	if strings.TrimSpace(username) == "" || !utf8.ValidString(username) || utf8.RuneCountInString(username) > 100 || len(password) < 12 || len(password) > 72 {
		return errors.New("username must be 1–100 characters; password must be 12–72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Create(&User{ID: Token(), Username: username, PasswordHash: string(hash), CreatedAt: time.Now().UTC()}).Error
}

// Fixed cost dummy hash keeps unknown users on the same password-check path.
const dummyHash = "$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW"

func (s Service) Login(ctx context.Context, username, password string) (User, string, error) {
	var user User
	err := s.DB.WithContext(ctx).Where("username=?", username).Take(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, "", err
	}
	hash := user.PasswordHash
	if err != nil {
		hash = dummyHash
	}
	check := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil || check != nil {
		return User{}, "", ErrUnauthorized
	}
	token := Token()
	now := time.Now().UTC()
	err = s.DB.WithContext(ctx).Create(&Session{TokenHash: Hash(token), AdminID: user.ID, CreatedAt: now, ExpiresAt: now.Add(s.TTL)}).Error
	return user, token, err
}
func (s Service) Current(ctx context.Context, token string) (User, error) {
	if len(token) != 64 {
		return User{}, ErrUnauthorized
	}
	var user User
	err := s.DB.WithContext(ctx).Table("admin_users u").Select("u.id,u.username").Joins("JOIN sessions s ON s.admin_id=u.id").Where("s.token_hash=? AND s.expires_at>?", Hash(token), time.Now().UTC()).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrUnauthorized
	}
	return user, err
}
func (s Service) Logout(ctx context.Context, token string) error {
	return s.DB.WithContext(ctx).Model(&Session{}).Where("token_hash=?", Hash(token)).Update("expires_at", time.Now().UTC()).Error
}

// A bounded global window suits a single-owner site; no untrusted proxy IPs.
type Limiter struct {
	mu     sync.Mutex
	start  time.Time
	count  int
	Limit  int
	Window time.Duration
}

func (l *Limiter) Allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.start) >= l.Window {
		l.start = now
		l.count = 0
	}
	if l.count >= l.Limit {
		return false
	}
	l.count++
	return true
}
