package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	authPath        = filepath.Join("data", "auth.json")
	authMutex       = &sync.RWMutex{}
	bcryptRounds    = 12
	maxOtpAttempts  = 5
	emailRegex      = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

type AuthUser struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	PasswordHash     string `json:"passwordHash"`
	Token            string `json:"token"`
	ResetOtpHash     *string `json:"resetOtpHash"`
	ResetOtpExpires  *int64 `json:"resetOtpExpires"`
	ResetOtpAttempts *int   `json:"resetOtpAttempts"`
	CreatedAt        string `json:"createdAt"`
}

type AuthData struct {
	Users []AuthUser `json:"users"`
}

// ── Helpers ──

func loadAuth() AuthData {
	authMutex.RLock()
	defer authMutex.RUnlock()

	var data AuthData
	data.Users = make([]AuthUser, 0)

	if _, err := os.Stat(authPath); os.IsNotExist(err) {
		return data
	}

	bytes, err := os.ReadFile(authPath)
	if err != nil {
		return data
	}

	json.Unmarshal(bytes, &data)
	if data.Users == nil {
		data.Users = make([]AuthUser, 0)
	}
	return data
}

func saveAuth(data AuthData) {
	authMutex.Lock()
	defer authMutex.Unlock()

	os.MkdirAll(filepath.Dir(authPath), 0755)
	bytes, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(authPath, bytes, 0644)
}

func generateToken() string {
	b := make([]byte, 48)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateOtp() string {
	val, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "123456" // Fallback if rand fails, though rare
	}
	return fmt.Sprintf("%06d", val.Int64()+100000)
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ── Public API ──

func HasAnyUsers() bool {
	auth := loadAuth()
	return len(auth.Users) > 0
}

func FindUserByEmail(email string) *AuthUser {
	auth := loadAuth()
	normalized := strings.TrimSpace(strings.ToLower(email))
	for i := range auth.Users {
		if auth.Users[i].Email == normalized {
			return &auth.Users[i]
		}
	}
	return nil
}

func FindUserByToken(token string) *AuthUser {
	if token == "" {
		return nil
	}
	auth := loadAuth()
	tokenBytes := []byte(token)

	for _, u := range auth.Users {
		storedBytes := []byte(u.Token)
		if len(tokenBytes) == len(storedBytes) && subtle.ConstantTimeCompare(tokenBytes, storedBytes) == 1 {
			return &u
		}
	}
	return nil
}

func Register(email, password string) (*AuthUser, error) {
	if email == "" || password == "" {
		return nil, errors.New("Email and password are required")
	}
	normalized := strings.TrimSpace(strings.ToLower(email))

	if !emailRegex.MatchString(normalized) {
		return nil, errors.New("Please enter a valid email address")
	}
	if len(password) < 6 {
		return nil, errors.New("Password must be at least 6 characters")
	}

	auth := loadAuth()
	for _, u := range auth.Users {
		if u.Email == normalized {
			return nil, errors.New("An account with this email already exists")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptRounds)
	if err != nil {
		return nil, err
	}

	user := AuthUser{
		ID:           generateUUID(),
		Email:        normalized,
		PasswordHash: string(hash),
		Token:        generateToken(),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	auth.Users = append(auth.Users, user)
	saveAuth(auth)

	return &user, nil
}

func Login(email, password string) (*AuthUser, error) {
	normalized := strings.TrimSpace(strings.ToLower(email))

	if normalized == "" || password == "" {
		return nil, errors.New("Email and password are required")
	}

	auth := loadAuth()
	var user *AuthUser
	for i, u := range auth.Users {
		if u.Email == normalized {
			user = &auth.Users[i]
			break
		}
	}

	if user == nil {
		return nil, errors.New("Invalid email or password")
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("Invalid email or password")
	}

	return user, nil
}

func ForgotPassword(email string) error {
	normalized := strings.TrimSpace(strings.ToLower(email))

	auth := loadAuth()
	foundIdx := -1
	for i, u := range auth.Users {
		if u.Email == normalized {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		return nil // Silently succeed
	}

	otp := generateOtp()
	hash := sha256.Sum256([]byte(otp))
	hashStr := hex.EncodeToString(hash[:])

	expires := time.Now().UnixMilli() + 15*60*1000
	attempts := 0

	auth.Users[foundIdx].ResetOtpHash = &hashStr
	auth.Users[foundIdx].ResetOtpExpires = &expires
	auth.Users[foundIdx].ResetOtpAttempts = &attempts
	saveAuth(auth)

	fmt.Printf("\n🔑 Password reset OTP for %s: %s\n", normalized, otp)
	fmt.Printf("   Valid for 15 minutes.\n\n")
	return nil
}

func ResetPassword(email, otp, newPassword string) (*AuthUser, error) {
	normalized := strings.TrimSpace(strings.ToLower(email))

	if otp == "" || newPassword == "" {
		return nil, errors.New("OTP and new password are required")
	}
	if len(newPassword) < 6 {
		return nil, errors.New("Password must be at least 6 characters")
	}

	auth := loadAuth()
	foundIdx := -1
	for i, u := range auth.Users {
		if u.Email == normalized {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		return nil, errors.New("Invalid email or OTP")
	}
	user := &auth.Users[foundIdx]

	attempts := 0
	if user.ResetOtpAttempts != nil {
		attempts = *user.ResetOtpAttempts
	}

	if attempts >= maxOtpAttempts {
		user.ResetOtpHash = nil
		user.ResetOtpExpires = nil
		user.ResetOtpAttempts = nil
		saveAuth(auth)
		return nil, errors.New("Too many attempts — please request a new reset code")
	}

	otpHashObj := sha256.Sum256([]byte(otp))
	otpHash := hex.EncodeToString(otpHashObj[:])

	if user.ResetOtpHash == nil || *user.ResetOtpHash != otpHash {
		attempts++
		user.ResetOtpAttempts = &attempts
		saveAuth(auth)
		return nil, errors.New("Invalid email or OTP")
	}

	if user.ResetOtpExpires == nil || time.Now().UnixMilli() > *user.ResetOtpExpires {
		user.ResetOtpHash = nil
		user.ResetOtpExpires = nil
		user.ResetOtpAttempts = nil
		saveAuth(auth)
		return nil, errors.New("OTP has expired — please request a new one")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptRounds)
	if err != nil {
		return nil, err
	}

	user.PasswordHash = string(newHash)
	user.Token = generateToken()
	user.ResetOtpHash = nil
	user.ResetOtpExpires = nil
	user.ResetOtpAttempts = nil
	saveAuth(auth)

	return user, nil
}
