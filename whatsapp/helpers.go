package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// maxMediaBytes caps how much data we will pull from a remote URL / decode from base64.
const maxMediaBytes = 100 * 1024 * 1024 // 100 MB

// defaultOpTimeout is applied to every whatsmeow request issued from the API layer.
const defaultOpTimeout = 90 * time.Second

// mediaFetchSem bounds how many remote media fetches can run concurrently so a
// burst of large URL fetches can't exhaust memory.
var mediaFetchSem = make(chan struct{}, 8)

// isBlockedIP reports whether an IP should be refused for outbound media fetches
// (SSRF guard against loopback / private / link-local / cloud-metadata targets).
func isBlockedIP(ip net.IP) bool {
	return ip == nil ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// mediaHTTPClient fetches media passed to the API as a URL. The dialer's Control
// hook runs against the post-DNS resolved address on the original request AND on
// every redirect, so it blocks DNS-rebinding and redirect-to-internal SSRF.
var mediaHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 15 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				if isBlockedIP(net.ParseIP(host)) {
					return fmt.Errorf("refusing to connect to non-public address")
				}
				return nil
			},
		}).DialContext,
	},
}

// decodedLen returns the approximate decoded byte length of a base64 string.
func decodedLen(b64 string) int {
	return len(b64) / 4 * 3
}

// requireClient returns the connected whatsmeow client for the user or an error
// if the session is not currently connected.
func requireClient(userId string) (*whatsmeow.Client, error) {
	uc := GetUserClient(userId)
	if uc.Client == nil || !uc.Client.IsConnected() {
		return nil, fmt.Errorf("WhatsApp client is not connected")
	}
	return uc.Client, nil
}

// opCtx builds a context with the default operation timeout.
func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultOpTimeout)
}

// ownJID returns the bare (non-AD) JID of the logged in account, or an empty JID.
func ownJID(cli *whatsmeow.Client) types.JID {
	if cli == nil || cli.Store == nil || cli.Store.ID == nil {
		return types.EmptyJID
	}
	return cli.Store.ID.ToNonAD()
}

// normalizeNumber strips a leading "+" and surrounding whitespace from a phone number.
func normalizeNumber(raw string) string {
	return strings.TrimPrefix(strings.TrimSpace(raw), "+")
}

// parseRecipient converts a user supplied destination into a JID. A value
// containing "@" is parsed as a full JID; anything else is treated as a phone
// number on the default user server.
func parseRecipient(raw string) (types.JID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return types.EmptyJID, fmt.Errorf("recipient is required")
	}
	if strings.ContainsRune(raw, '@') {
		return types.ParseJID(raw)
	}
	return types.NewJID(normalizeNumber(raw), types.DefaultUserServer), nil
}

// parseGroupJID converts a user supplied group id into a JID on the group server.
func parseGroupJID(raw string) (types.JID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return types.EmptyJID, fmt.Errorf("group id is required")
	}
	if strings.ContainsRune(raw, '@') {
		return types.ParseJID(raw)
	}
	return types.NewJID(raw, types.GroupServer), nil
}

// parseNewsletterJID converts a user supplied channel id into a JID on the newsletter server.
func parseNewsletterJID(raw string) (types.JID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return types.EmptyJID, fmt.Errorf("newsletter id is required")
	}
	if strings.ContainsRune(raw, '@') {
		return types.ParseJID(raw)
	}
	return types.NewJID(raw, types.NewsletterServer), nil
}

// parseJIDList parses a slice of raw recipient strings into JIDs.
func parseJIDList(raw []string) ([]types.JID, error) {
	jids := make([]types.JID, 0, len(raw))
	for _, r := range raw {
		jid, err := parseRecipient(r)
		if err != nil {
			return nil, err
		}
		jids = append(jids, jid)
	}
	return jids, nil
}

// cleanMime trims parameters such as "; charset=..." from a mime type.
func cleanMime(mime string) string {
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i]
	}
	return strings.TrimSpace(mime)
}

// resolveMedia accepts a data URI (data:<mime>;base64,...), an http(s) URL, or a
// raw base64 string and returns the decoded bytes together with a best effort mime type.
func resolveMedia(source string) (data []byte, mime string, err error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, "", fmt.Errorf("media source is empty")
	}

	switch {
	case strings.HasPrefix(source, "data:"):
		comma := strings.IndexByte(source, ',')
		if comma < 0 {
			return nil, "", fmt.Errorf("invalid data URI")
		}
		meta := source[len("data:"):comma]
		payload := source[comma+1:]
		if decodedLen(payload) > maxMediaBytes {
			return nil, "", fmt.Errorf("media exceeds the %d MB limit", maxMediaBytes/1024/1024)
		}
		mime = "application/octet-stream"
		if meta != "" {
			mime = cleanMime(meta)
		}
		data, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", fmt.Errorf("invalid base64 in data URI")
		}
		return data, mime, nil

	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		mediaFetchSem <- struct{}{}
		defer func() { <-mediaFetchSem }()

		resp, err := mediaHTTPClient.Get(source)
		if err != nil {
			// Don't leak the underlying network error (it would otherwise act as an
			// internal-network port-scan oracle for the SSRF-blocked dialer).
			return nil, "", fmt.Errorf("failed to fetch media URL")
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, "", fmt.Errorf("failed to fetch media: HTTP %d", resp.StatusCode)
		}
		data, err = io.ReadAll(io.LimitReader(resp.Body, maxMediaBytes))
		if err != nil {
			return nil, "", fmt.Errorf("failed to read media response")
		}
		mime = cleanMime(resp.Header.Get("Content-Type"))
		if mime == "" {
			mime = http.DetectContentType(data)
		}
		return data, mime, nil

	default:
		if decodedLen(source) > maxMediaBytes {
			return nil, "", fmt.Errorf("media exceeds the %d MB limit", maxMediaBytes/1024/1024)
		}
		data, err = base64.StdEncoding.DecodeString(source)
		if err != nil {
			return nil, "", fmt.Errorf("media must be a URL, data URI, or base64 string")
		}
		return data, http.DetectContentType(data), nil
	}
}

// canonicalChatJID returns the conversation key for an incoming message,
// normalizing a LID-addressed 1:1 chat to the contact's phone-number JID so a
// contact's messages never split into a "@lid" and a "@s.whatsapp.net" thread.
// Group chats and already-phone chats are returned unchanged.
func canonicalChatJID(cli *whatsmeow.Client, info *types.MessageInfo) types.JID {
	chat := info.Chat
	if info.IsGroup || chat.Server != types.HiddenUserServer {
		return chat
	}
	// Prefer the phone alternative the server already attached to the message.
	if info.SenderAlt.Server == types.DefaultUserServer && info.SenderAlt.User != "" {
		return info.SenderAlt
	}
	// Otherwise consult the local LID↔PN mapping store.
	if cli != nil && cli.Store != nil && cli.Store.LIDs != nil {
		if pn, err := cli.Store.LIDs.GetPNForLID(context.Background(), chat.ToNonAD()); err == nil && pn.User != "" {
			return pn
		}
	}
	return chat
}

// jidString safely returns the string form of a JID, or "" for an empty JID.
func jidString(jid types.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	return jid.String()
}
