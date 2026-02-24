package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wa-server-go/storage"

	
	_ "github.com/glebarez/sqlite"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"net/http"
)

// ── Per-user client instances ──

type ClientState struct {
	Client           *whatsmeow.Client  `json:"-"`
	ConnectionStatus string             `json:"status"`
	PairingCode      *string            `json:"pairingCode"`
	QRCodeData       *string            `json:"qr"`
	ClientInfo       *ClientInfo        `json:"info"`
	LastError        *string            `json:"error"`
	CancelPairing    context.CancelFunc `json:"-"`
}

type ClientInfo struct {
	PushName string `json:"pushname"`
	Phone    string `json:"phone"`
	Platform string `json:"platform"`
}

var (
	userClients = make(map[string]*ClientState)
	clientsLock = sync.RWMutex{}
	log         = waLog.Stdout("INFO", "WARN", true)
	dbContainer *sqlstore.Container
)

func init() {
	// whatsmeow requires a SQLite database to store sessions
	os.MkdirAll("data", 0755)
	var err error
	// Use PRAGMAs to handle concurrent access
	dsn := "file:data/whatsapp_sessions.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	dbContainer, err = sqlstore.New(context.Background(), "sqlite", dsn, log)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize SQLite for WhatsApp: %v", err))
	}
}

func GetUserClient(userId string) *ClientState {
	clientsLock.RLock()
	uc, ok := userClients[userId]
	clientsLock.RUnlock()

	if !ok {
		clientsLock.Lock()
		uc, ok = userClients[userId]
		if !ok {
			uc = &ClientState{
				ConnectionStatus: "disconnected",
			}
			userClients[userId] = uc
		}
		clientsLock.Unlock()
	}
	return uc
}



// ── Event Handler ──

func eventHandler(userId string, client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			if v.Info.IsFromMe {
				return
			}
			// Build message data matching JS format
			contactName := v.Info.PushName
			if contactName == "" {
				contactName = v.Info.Sender.User
			}

			isGroup := v.Info.IsGroup
			var groupName *string
			if isGroup {
				g := v.Info.Sender.User // fallback
				// To get real group name we'd need to query group info, omitting for speed or fetch from cache
				groupName = &g
			}

			var body string
			if v.Message.GetConversation() != "" {
				body = v.Message.GetConversation()
			} else if v.Message.ExtendedTextMessage != nil {
				body = v.Message.ExtendedTextMessage.GetText()
			} else {
				body = "Media/Other Message"
			}

			messageData := map[string]interface{}{
				"id":          v.Info.ID,
				"from":        v.Info.Sender.ToNonAD().String(),
				"to":          userId, // Not technically correct, but mimicking JS 'to'
				"body":        body,
				"timestamp":   v.Info.Timestamp.UTC().Format(time.RFC3339),
				"type":        "received",
				"contactName": contactName,
				"isGroup":     isGroup,
				"groupName":   groupName,
			}

			storage.PushToUserMessage(userId, messageData)
			storage.IncrementStatUser(userId, "messagesReceived")

			// Fire webhooks
			userData := storage.LoadUser(userId)
			for _, hook := range userData.Webhooks {
				hookMap, ok := hook.(map[string]interface{})
				if !ok {
					continue
				}
				urlStr, ok := hookMap["url"].(string)
				if !ok {
					continue
				}

				payload, _ := json.Marshal(messageData)
				go func(url string, body []byte) {
					resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
					if err != nil {
						fmt.Printf("Webhook failed (%s): %v\n", url, err)
					} else if resp != nil {
						resp.Body.Close()
					}
				}(urlStr, payload)
			}

		case *events.Connected:
			uc := GetUserClient(userId)
			uc.ConnectionStatus = "ready"
			uc.PairingCode = nil
			uc.QRCodeData = nil
			if client.Store.ID != nil {
				uc.ClientInfo = &ClientInfo{
					PushName: client.Store.PushName,
					Phone:    client.Store.ID.User,
					Platform: "whatsmeow",
				}
				fmt.Printf("✅ [%.8s] WhatsApp connected as %s (%s)\n", userId, uc.ClientInfo.PushName, uc.ClientInfo.Phone)
			}

		case *events.Disconnected:
			uc := GetUserClient(userId)
			uc.ConnectionStatus = "disconnected"
			uc.PairingCode = nil
			uc.QRCodeData = nil
			uc.ClientInfo = nil
			storage.ClearUserBotData(userId)
			fmt.Printf("❌ [%.8s] WhatsApp disconnected\n", userId)

		case *events.LoggedOut:
			uc := GetUserClient(userId)
			uc.ConnectionStatus = "disconnected"
			storage.ClearUserBotData(userId)
			client.Disconnect()

		case *events.PairSuccess:
			fmt.Printf("✅ [%.8s] Pairing successful!\n", userId)
		}
	}
}

// ── Operations ──

func Initialize(userId string, method string, phoneNumber string) error {
	uc := GetUserClient(userId)

	if uc.Client != nil {
		uc.Client.Disconnect()
		uc.Client = nil
	}

	uc.ConnectionStatus = "initializing"
	uc.PairingCode = nil
	uc.QRCodeData = nil
	uc.ClientInfo = nil
	uc.LastError = nil

	if uc.CancelPairing != nil {
		uc.CancelPairing()
		uc.CancelPairing = nil
	}

	// Create user-specific database container
	dbPath := filepath.Join("data", "users", userId, "session.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	container, err := sqlstore.New(context.Background(), "sqlite", dsn, log)
	if err != nil {
		errStr := err.Error()
		uc.LastError = &errStr
		uc.ConnectionStatus = "error"
		return err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	client := whatsmeow.NewClient(deviceStore, log)
	uc.Client = client
	client.AddEventHandler(eventHandler(userId, client))

	if client.Store.ID == nil {
		// New login
		if method == "pairing_code" {
			uc.ConnectionStatus = "pairing_code"
			if phoneNumber != "" {
				err = client.Connect()
				if err != nil {
					errStr := err.Error()
					uc.LastError = &errStr
					uc.ConnectionStatus = "error"
					return err
				}

				code, err := client.PairPhone(context.Background(), phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Windows)")
				if err != nil {
					errStr := err.Error()
					uc.LastError = &errStr
					uc.ConnectionStatus = "error"
					return err
				}
				uc.PairingCode = &code
				fmt.Printf("📱 [%.8s] Pairing code for %s: %s\n", userId, phoneNumber, code)
			} else {
				errStr := "Phone number is required for pairing code"
				uc.LastError = &errStr
				uc.ConnectionStatus = "error"
			}
		} else {
			// QR
			qrChan, _ := client.GetQRChannel(context.Background())
			err = client.Connect()
			if err != nil {
				return err
			}
			uc.ConnectionStatus = "qr"
			go func() {
				for evt := range qrChan {
					if evt.Event == "code" {
						qrImage, _ := qrcode.Encode(evt.Code, qrcode.Medium, 256)
						b64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrImage)
						uc.QRCodeData = &b64
						fmt.Printf("📱 [%.8s] QR code generated, scan to connect\n", userId)
					}
				}
			}()
		}
	} else {
		// Already logged in
		err = client.Connect()
		if err != nil {
			errStr := err.Error()
			uc.LastError = &errStr
			uc.ConnectionStatus = "error"
			return err
		}
		uc.ConnectionStatus = "ready"
	}

	return nil
}

func Disconnect(userId string) error {
	uc := GetUserClient(userId)
	if uc.Client != nil {
		uc.Client.Logout(context.Background())
		uc.Client.Disconnect()
		uc.Client = nil
	}
	uc.ConnectionStatus = "disconnected"
	uc.PairingCode = nil
	uc.QRCodeData = nil
	uc.ClientInfo = nil
	uc.LastError = nil
	storage.ClearUserBotData(userId)
	fmt.Printf("🔌 [%.8s] WhatsApp disconnected by user\n", userId)
	return nil
}

// --- Endpoints mapping ---

func SendMessage(userId string, number string, message string) (interface{}, error) {
	uc := GetUserClient(userId)
	if uc.Client == nil || !uc.Client.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jid := types.NewJID(number, types.DefaultUserServer)
	if resp, err := uc.Client.IsOnWhatsApp(context.Background(), []string{jid.String()}); err == nil && len(resp) > 0 {
		if !resp[0].IsIn {
			// Just assuming it works for now
		}
	}

	msgId := whatsmeow.GenerateMessageID()
	resp, err := uc.Client.SendMessage(context.Background(), jid, &waProto.Message{
		Conversation: &message,
	})

	if err != nil {
		return nil, err
	}

	messageData := map[string]interface{}{
		"id":          msgId,
		"from":        "me",
		"to":          jid.String(),
		"body":        message,
		"timestamp":   resp.Timestamp.UTC().Format(time.RFC3339),
		"type":        "sent",
		"contactName": number,
		"isGroup":     false,
		"groupName":   nil,
	}

	storage.PushToUserMessage(userId, messageData)
	storage.IncrementStatUser(userId, "messagesSent")

	return messageData, nil
}

func SendGroupMessage(userId string, groupId string, message string) (interface{}, error) {
	uc := GetUserClient(userId)
	if uc.Client == nil || !uc.Client.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jid := types.NewJID(groupId, types.GroupServer)
	msgId := whatsmeow.GenerateMessageID()
	resp, err := uc.Client.SendMessage(context.Background(), jid, &waProto.Message{
		Conversation: &message,
	})

	if err != nil {
		return nil, err
	}

	messageData := map[string]interface{}{
		"id":          msgId,
		"from":        "me",
		"to":          jid.String(),
		"body":        message,
		"timestamp":   resp.Timestamp.UTC().Format(time.RFC3339),
		"type":        "sent",
		"contactName": "Group",
		"isGroup":     true,
		"groupName":   groupId,
	}

	storage.PushToUserMessage(userId, messageData)
	storage.IncrementStatUser(userId, "messagesSent")

	return messageData, nil
}

func GetGroups(userId string) ([]interface{}, error) {
	uc := GetUserClient(userId)
	if uc.Client == nil || !uc.Client.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	groups, err := uc.Client.GetJoinedGroups(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, 0)
	for _, g := range groups {
		result = append(result, map[string]interface{}{
			"id":               g.JID.User,
			"name":             g.Name,
			"participantCount": len(g.Participants),
			"isReadOnly":       g.IsAnnounce,
		})
	}
	return result, nil
}

func JoinGroup(userId string, inviteCode string) (interface{}, error) {
	uc := GetUserClient(userId)
	if uc.Client == nil || !uc.Client.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jid, err := uc.Client.JoinGroupWithLink(context.Background(), inviteCode)
	if err != nil {
		return nil, err
	}
	storage.IncrementStatUser(userId, "groupsJoined")
	return map[string]interface{}{"success": true, "groupId": jid.String()}, nil
}

func LeaveGroup(userId string, groupId string) (interface{}, error) {
	uc := GetUserClient(userId)
	if uc.Client == nil || !uc.Client.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jid := types.NewJID(groupId, types.GroupServer)
	err := uc.Client.LeaveGroup(context.Background(), jid)
	if err != nil {
		return nil, err
	}
	storage.IncrementStatUser(userId, "groupsLeft")
	return map[string]interface{}{"success": true, "groupId": groupId}, nil
}

func AddToGroup(userId string, groupId string, participants []string) (interface{}, error) {
	uc := GetUserClient(userId)
	if uc.Client == nil || !uc.Client.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jids := make([]types.JID, 0)
	for _, p := range participants {
		jids = append(jids, types.NewJID(p, types.DefaultUserServer))
	}

	groupID := types.NewJID(groupId, types.GroupServer)
	_, err := uc.Client.UpdateGroupParticipants(context.Background(), groupID, jids, whatsmeow.ParticipantChangeAdd)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true}, nil
}
