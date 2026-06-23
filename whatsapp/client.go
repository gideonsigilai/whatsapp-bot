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
	mu               sync.RWMutex       `json:"-"`
	Client           *whatsmeow.Client  `json:"-"`
	ConnectionStatus string             `json:"status"`
	PairingCode      *string            `json:"pairingCode"`
	QRCodeData       *string            `json:"qr"`
	ClientInfo       *ClientInfo        `json:"info"`
	LastError        *string            `json:"error"`
	CancelPairing    context.CancelFunc `json:"-"`
}

// update runs a mutation of the client state fields under the write lock.
func (uc *ClientState) update(fn func()) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	fn()
}

// getClient returns the current whatsmeow client under the read lock.
func (uc *ClientState) getClient() *whatsmeow.Client {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.Client
}

// snapshot returns a JSON-friendly copy of the visible state under the read lock.
func (uc *ClientState) snapshot() map[string]interface{} {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return map[string]interface{}{
		"status":      uc.ConnectionStatus,
		"pairingCode": uc.PairingCode,
		"qr":          uc.QRCodeData,
		"info":        uc.ClientInfo,
		"error":       uc.LastError,
	}
}

// setError records an error state under the write lock.
func (uc *ClientState) setError(err error) {
	uc.update(func() {
		s := err.Error()
		uc.LastError = &s
		uc.ConnectionStatus = "error"
	})
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

	// Webhook delivery uses a shared client with a timeout and a bounded number of
	// in-flight requests so a slow/hung endpoint can't leak goroutines or sockets.
	webhookClient = &http.Client{Timeout: 10 * time.Second}
	webhookSem    = make(chan struct{}, 32)
)

// deliverWebhook POSTs a payload to a webhook URL with a timeout and concurrency bound.
func deliverWebhook(url string, body []byte) {
	webhookSem <- struct{}{}
	defer func() { <-webhookSem }()

	resp, err := webhookClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("Webhook failed (%s): %v\n", url, err)
		return
	}
	resp.Body.Close()
}

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

// GetStatus returns a snapshot of a user's connection state, suitable for the
// /api/status endpoint and for the initial websocket "status" event.
func GetStatus(userId string) map[string]interface{} {
	uc := GetUserClient(userId)
	cli := uc.getClient()
	snap := uc.snapshot()
	snap["connected"] = cli != nil && cli.IsConnected()
	snap["sockets"] = ConnectedSockets(userId)
	return snap
}

// broadcastStatus pushes the current status snapshot to all of a user's sockets.
func broadcastStatus(userId string) {
	Broadcast(userId, "status", GetStatus(userId))
}

// ── Event Handler ──

func eventHandler(userId string, client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			// Status/story broadcasts go to the dedicated status feed, not the inbox.
			if v.Info.Chat.Server == types.BroadcastServer && v.Info.Chat.User == "status" {
				if !v.Info.IsFromMe {
					recordStatusUpdate(userId, v)
				}
				return
			}
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

			// Unwrap ephemeral / view-once / device-sent envelopes so text and
			// media are read from the real inner message, not an empty wrapper.
			inner := unwrapMessage(v.Message)

			var body string
			hasMedia := false
			var mediaType, mediaMime, mediaFilename string
			if cm, ok := mediaInfo(inner, v.Info.ID); ok {
				// Cache the (unwrapped) message so the media can be downloaded on demand.
				cacheMedia(userId, v.Info.ID, cm)
				hasMedia = true
				mediaType = cm.mediaType
				mediaMime = cm.mimetype
				mediaFilename = cm.filename
				if cm.caption != "" {
					body = cm.caption
				} else {
					body = "[" + cm.mediaType + "]"
				}
			} else if b := extractBody(inner); b != "" {
				body = b
			} else {
				body = "Media/Other Message"
			}

			// Normalize a LID-addressed 1:1 chat to the contact's phone JID so the
			// conversation doesn't split across "@lid" and "@s.whatsapp.net".
			chatJID := canonicalChatJID(client, &v.Info)
			chatStr := chatJID.ToNonAD().String()
			fromStr := v.Info.Sender.ToNonAD().String()
			if !isGroup {
				fromStr = chatStr
			}

			messageData := map[string]interface{}{
				"id":            v.Info.ID,
				"chat":          chatStr,
				"from":          fromStr,
				"to":            userId, // Not technically correct, but mimicking JS 'to'
				"body":          body,
				"timestamp":     v.Info.Timestamp.UTC().Format(time.RFC3339),
				"type":          "received",
				"contactName":   contactName,
				"isGroup":       isGroup,
				"groupName":     groupName,
				"hasMedia":      hasMedia,
				"mediaType":     mediaType,
				"mediaMime":     mediaMime,
				"mediaFilename": mediaFilename,
			}

			storage.PushToUserMessage(userId, messageData)
			storage.IncrementStatUser(userId, "messagesReceived")

			// Realtime push to connected websocket clients
			Broadcast(userId, "message", messageData)

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
				go deliverWebhook(urlStr, payload)
			}

		case *events.Connected:
			uc := GetUserClient(userId)
			var info *ClientInfo
			if client.Store.ID != nil {
				info = &ClientInfo{
					PushName: client.Store.PushName,
					Phone:    client.Store.ID.User,
					Platform: "whatsmeow",
				}
			}
			uc.update(func() {
				uc.ConnectionStatus = "ready"
				uc.PairingCode = nil
				uc.QRCodeData = nil
				if info != nil {
					uc.ClientInfo = info
				}
			})
			if info != nil {
				fmt.Printf("✅ [%.8s] WhatsApp connected as %s (%s)\n", userId, info.PushName, info.Phone)
			}
			broadcastStatus(userId)

		case *events.Disconnected:
			uc := GetUserClient(userId)
			uc.update(func() {
				uc.ConnectionStatus = "disconnected"
				uc.PairingCode = nil
				uc.QRCodeData = nil
				uc.ClientInfo = nil
			})
			storage.ClearUserBotData(userId)
			fmt.Printf("❌ [%.8s] WhatsApp disconnected\n", userId)
			broadcastStatus(userId)

		case *events.LoggedOut:
			uc := GetUserClient(userId)
			uc.update(func() { uc.ConnectionStatus = "disconnected" })
			storage.ClearUserBotData(userId)
			client.Disconnect()
			broadcastStatus(userId)

		case *events.PairSuccess:
			fmt.Printf("✅ [%.8s] Pairing successful!\n", userId)
			Broadcast(userId, "pair_success", map[string]interface{}{"jid": v.ID.String()})

		case *events.Receipt:
			Broadcast(userId, "receipt", map[string]interface{}{
				"chat":       v.Chat.String(),
				"sender":     v.Sender.String(),
				"type":       string(v.Type),
				"messageIds": v.MessageIDs,
				"timestamp":  v.Timestamp.UTC().Format(time.RFC3339),
			})

		case *events.Presence:
			Broadcast(userId, "presence", map[string]interface{}{
				"from":        v.From.String(),
				"unavailable": v.Unavailable,
				"lastSeen":    v.LastSeen.UTC().Format(time.RFC3339),
			})

		case *events.ChatPresence:
			Broadcast(userId, "chat_presence", map[string]interface{}{
				"chat":   v.Chat.String(),
				"sender": v.Sender.String(),
				"state":  string(v.State),
				"media":  string(v.Media),
			})

		case *events.GroupInfo:
			Broadcast(userId, "group_info", map[string]interface{}{
				"jid":       v.JID.String(),
				"timestamp": v.Timestamp.UTC().Format(time.RFC3339),
			})

		case *events.JoinedGroup:
			Broadcast(userId, "joined_group", groupInfoToMap(&v.GroupInfo))

		case *events.Picture:
			Broadcast(userId, "picture", map[string]interface{}{
				"jid":       v.JID.String(),
				"author":    v.Author.String(),
				"pictureId": v.PictureID,
				"removed":   v.Remove,
			})

		case *events.CallOffer:
			Broadcast(userId, "call", map[string]interface{}{
				"from":      v.From.String(),
				"creator":   v.CallCreator.String(),
				"callId":    v.CallID,
				"timestamp": v.Timestamp.UTC().Format(time.RFC3339),
			})
		}
	}
}

// ── Operations ──

func Initialize(userId string, method string, phoneNumber string) error {
	uc := GetUserClient(userId)

	// Tear down any previous client + pairing flow.
	if old := uc.getClient(); old != nil {
		old.Disconnect()
	}
	uc.update(func() {
		uc.Client = nil
		uc.ConnectionStatus = "initializing"
		uc.PairingCode = nil
		uc.QRCodeData = nil
		uc.ClientInfo = nil
		uc.LastError = nil
		if uc.CancelPairing != nil {
			uc.CancelPairing()
			uc.CancelPairing = nil
		}
	})

	// Create user-specific database container
	dbPath := filepath.Join("data", "users", userId, "session.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	container, err := sqlstore.New(context.Background(), "sqlite", dsn, log)
	if err != nil {
		uc.setError(err)
		return err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	client := whatsmeow.NewClient(deviceStore, log)
	uc.update(func() { uc.Client = client })
	client.AddEventHandler(eventHandler(userId, client))

	if client.Store.ID == nil {
		// New login
		if method == "pairing_code" {
			uc.update(func() { uc.ConnectionStatus = "pairing_code" })
			if phoneNumber != "" {
				if err = client.Connect(); err != nil {
					uc.setError(err)
					return err
				}

				code, err := client.PairPhone(context.Background(), phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Windows)")
				if err != nil {
					uc.setError(err)
					return err
				}
				uc.update(func() { uc.PairingCode = &code })
				fmt.Printf("📱 [%.8s] Pairing code for %s: %s\n", userId, phoneNumber, code)
				broadcastStatus(userId)
			} else {
				uc.update(func() {
					errStr := "Phone number is required for pairing code"
					uc.LastError = &errStr
					uc.ConnectionStatus = "error"
				})
			}
		} else {
			// QR — use a cancelable context so a later Initialize/Disconnect stops
			// this reader goroutine instead of leaking it.
			ctx, cancel := context.WithCancel(context.Background())
			uc.update(func() { uc.CancelPairing = cancel })

			qrChan, _ := client.GetQRChannel(ctx)
			if err = client.Connect(); err != nil {
				cancel()
				uc.setError(err)
				return err
			}
			uc.update(func() { uc.ConnectionStatus = "qr" })
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case evt, ok := <-qrChan:
						if !ok {
							return
						}
						if evt.Event == "code" {
							qrImage, _ := qrcode.Encode(evt.Code, qrcode.Medium, 256)
							b64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrImage)
							uc.update(func() { uc.QRCodeData = &b64 })
							fmt.Printf("📱 [%.8s] QR code generated, scan to connect\n", userId)
							broadcastStatus(userId)
						}
					}
				}
			}()
		}
	} else {
		// Already logged in
		if err = client.Connect(); err != nil {
			uc.setError(err)
			return err
		}
		uc.update(func() { uc.ConnectionStatus = "ready" })
	}

	return nil
}

func Disconnect(userId string) error {
	uc := GetUserClient(userId)

	old := uc.getClient()
	var cancel context.CancelFunc
	uc.update(func() {
		uc.Client = nil
		uc.ConnectionStatus = "disconnected"
		uc.PairingCode = nil
		uc.QRCodeData = nil
		uc.ClientInfo = nil
		uc.LastError = nil
		cancel = uc.CancelPairing
		uc.CancelPairing = nil
	})
	if cancel != nil {
		cancel()
	}
	if old != nil {
		old.Logout(context.Background())
		old.Disconnect()
	}
	storage.ClearUserBotData(userId)
	fmt.Printf("🔌 [%.8s] WhatsApp disconnected by user\n", userId)
	return nil
}

// --- Endpoints mapping ---

func SendMessage(userId string, number string, message string) (interface{}, error) {
	uc := GetUserClient(userId)
	cli := uc.getClient()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jid := types.NewJID(number, types.DefaultUserServer)
	resp, err := cli.SendMessage(context.Background(), jid, &waProto.Message{
		Conversation: &message,
	})

	if err != nil {
		return nil, err
	}

	messageData := map[string]interface{}{
		"id":          string(resp.ID),
		"chat":        jid.String(),
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
	Broadcast(userId, "message", messageData)

	return messageData, nil
}

func SendGroupMessage(userId string, groupId string, message string) (interface{}, error) {
	uc := GetUserClient(userId)
	cli := uc.getClient()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jid := types.NewJID(groupId, types.GroupServer)
	resp, err := cli.SendMessage(context.Background(), jid, &waProto.Message{
		Conversation: &message,
	})

	if err != nil {
		return nil, err
	}

	messageData := map[string]interface{}{
		"id":          string(resp.ID),
		"chat":        jid.String(),
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
	Broadcast(userId, "message", messageData)

	return messageData, nil
}

func GetGroups(userId string) ([]interface{}, error) {
	uc := GetUserClient(userId)
	cli := uc.getClient()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	groups, err := cli.GetJoinedGroups(context.Background())
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
	cli := uc.getClient()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jid, err := cli.JoinGroupWithLink(context.Background(), inviteCode)
	if err != nil {
		return nil, err
	}
	storage.IncrementStatUser(userId, "groupsJoined")
	return map[string]interface{}{"success": true, "groupId": jid.String()}, nil
}

func LeaveGroup(userId string, groupId string) (interface{}, error) {
	uc := GetUserClient(userId)
	cli := uc.getClient()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jid := types.NewJID(groupId, types.GroupServer)
	err := cli.LeaveGroup(context.Background(), jid)
	if err != nil {
		return nil, err
	}
	storage.IncrementStatUser(userId, "groupsLeft")
	return map[string]interface{}{"success": true, "groupId": groupId}, nil
}

func AddToGroup(userId string, groupId string, participants []string) (interface{}, error) {
	uc := GetUserClient(userId)
	cli := uc.getClient()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}

	jids := make([]types.JID, 0)
	for _, p := range participants {
		jids = append(jids, types.NewJID(p, types.DefaultUserServer))
	}

	groupID := types.NewJID(groupId, types.GroupServer)
	_, err := cli.UpdateGroupParticipants(context.Background(), groupID, jids, whatsmeow.ParticipantChangeAdd)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true}, nil
}
