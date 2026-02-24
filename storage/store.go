package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var (
	dataDir  = "data"
	usersDir = filepath.Join(dataDir, "users")

	globalConfigPath = filepath.Join(dataDir, "global.json")

	globalMutex = &sync.RWMutex{}

	userLocks = struct {
		sync.RWMutex
		m map[string]*sync.RWMutex
	}{m: make(map[string]*sync.RWMutex)}
)

func init() {
	if err := os.MkdirAll(usersDir, 0755); err != nil {
		fmt.Printf("Failed to create users dir: %v\n", err)
	}
	EnsureGlobal()
}

// ── Helpers ──

func getUserLock(userId string) *sync.RWMutex {
	userLocks.RLock()
	lock, ok := userLocks.m[userId]
	userLocks.RUnlock()

	if !ok {
		userLocks.Lock()
		lock, ok = userLocks.m[userId]
		if !ok {
			lock = &sync.RWMutex{}
			userLocks.m[userId] = lock
		}
		userLocks.Unlock()
	}
	return lock
}

func sanitizeUserId(userId string) (string, error) {
	re := regexp.MustCompile(`[^a-zA-Z0-9\-]`)
	clean := re.ReplaceAllString(userId, "")
	if clean != userId || clean == "" {
		return "", fmt.Errorf("invalid user ID format")
	}
	return clean, nil
}

func UserDataPath(userId string) string {
	safeId, _ := sanitizeUserId(userId)
	return filepath.Join(usersDir, safeId, "data.json")
}

// ── Models ──

type GlobalConfig struct {
	Config struct {
		BotName       string `json:"botName"`
		Port          int    `json:"port"`
		TunnelEnabled bool   `json:"tunnelEnabled"`
	} `json:"config"`
}

type UserStats struct {
	MessagesSent     int `json:"messagesSent"`
	MessagesReceived int `json:"messagesReceived"`
	GroupsJoined     int `json:"groupsJoined"`
	GroupsLeft       int `json:"groupsLeft"`
}

// Flexible UserData model to match exactly what db.js produced
type UserData struct {
	Messages []interface{} `json:"messages"`
	Groups   []interface{} `json:"groups"`
	Webhooks []interface{} `json:"webhooks"`
	Stats    UserStats     `json:"stats"`
}

var DefaultUserData = UserData{
	Messages: make([]interface{}, 0),
	Groups:   make([]interface{}, 0),
	Webhooks: make([]interface{}, 0),
	Stats:    UserStats{},
}

var DefaultGlobal = GlobalConfig{
	Config: struct {
		BotName       string `json:"botName"`
		Port          int    `json:"port"`
		TunnelEnabled bool   `json:"tunnelEnabled"`
	}{
		BotName:       "WA Bot Server",
		Port:          3000,
		TunnelEnabled: false,
	},
}

// ── Global Config Methods ──

func EnsureGlobal() {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if _, err := os.Stat(globalConfigPath); os.IsNotExist(err) {
		saveGlobalConfigRaw(DefaultGlobal)
	}
}

func GetGlobalConfig() GlobalConfig {
	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if _, err := os.Stat(globalConfigPath); os.IsNotExist(err) {
		return DefaultGlobal
	}

	data, err := os.ReadFile(globalConfigPath)
	if err != nil {
		return DefaultGlobal
	}

	var conf GlobalConfig
	if err := json.Unmarshal(data, &conf); err != nil {
		return DefaultGlobal
	}
	return conf
}

func saveGlobalConfigRaw(conf GlobalConfig) {
	data, _ := json.MarshalIndent(conf, "", "  ")
	os.WriteFile(globalConfigPath, data, 0644)
}

// ── Per-User Methods ──

func InitUser(userId string) {
	safeId, err := sanitizeUserId(userId)
	if err != nil {
		return
	}
	p := UserDataPath(safeId)

	lock := getUserLock(safeId)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(p); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(p), 0755)
		data, _ := json.MarshalIndent(DefaultUserData, "", "  ")
		os.WriteFile(p, data, 0644)
	}
}

func LoadUser(userId string) UserData {
	safeId, err := sanitizeUserId(userId)
	if err != nil {
		return DefaultUserData
	}

	lock := getUserLock(safeId)
	lock.RLock()
	defer lock.RUnlock()

	p := UserDataPath(safeId)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		// unlock temporarily to init
		lock.RUnlock()
		InitUser(safeId)
		lock.RLock()

		return DefaultUserData
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return DefaultUserData
	}

	var ud UserData
	if err := json.Unmarshal(data, &ud); err != nil {
		return DefaultUserData
	}

	// ensure slices are not nil
	if ud.Messages == nil {
		ud.Messages = make([]interface{}, 0)
	}
	if ud.Groups == nil {
		ud.Groups = make([]interface{}, 0)
	}
	if ud.Webhooks == nil {
		ud.Webhooks = make([]interface{}, 0)
	}

	return ud
}

func SaveUser(userId string, data UserData) {
	safeId, err := sanitizeUserId(userId)
	if err != nil {
		return
	}

	lock := getUserLock(safeId)
	lock.Lock()
	defer lock.Unlock()

	p := UserDataPath(safeId)
	os.MkdirAll(filepath.Dir(p), 0755)

	bytes, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(p, bytes, 0644)
}

func PushToUserMessage(userId string, item interface{}) {
	data := LoadUser(userId)
	data.Messages = append(data.Messages, item)

	if len(data.Messages) > 500 {
		data.Messages = data.Messages[len(data.Messages)-500:]
	}

	SaveUser(userId, data)
}

func IncrementStatUser(userId string, statKey string) {
	data := LoadUser(userId)
	switch statKey {
	case "messagesSent":
		data.Stats.MessagesSent++
	case "messagesReceived":
		data.Stats.MessagesReceived++
	case "groupsJoined":
		data.Stats.GroupsJoined++
	case "groupsLeft":
		data.Stats.GroupsLeft++
	}
	SaveUser(userId, data)
}

func ClearUserBotData(userId string) {
	data := LoadUser(userId)
	data.Messages = make([]interface{}, 0)
	data.Webhooks = make([]interface{}, 0)
	data.Stats = UserStats{}
	SaveUser(userId, data)
}

func RegisterWebhook(userId string, hook map[string]interface{}) {
	data := LoadUser(userId)
	data.Webhooks = append(data.Webhooks, hook)
	SaveUser(userId, data)
}

func UnregisterWebhook(userId string, hookId string) {
	data := LoadUser(userId)
	var newHooks []interface{}
	for _, h := range data.Webhooks {
		hw := h.(map[string]interface{})
		if fmt.Sprintf("%v", hw["id"]) != hookId {
			newHooks = append(newHooks, h)
		}
	}
	data.Webhooks = newHooks
	SaveUser(userId, data)
}

func GetWebhooks(userId string) []interface{} {
	data := LoadUser(userId)
	return data.Webhooks
}

