package whatsapp

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// ── Incoming media cache ──
//
// whatsmeow can only download a media message if it still has the original
// protobuf (which carries the media keys + direct path). The on-disk message
// log only stores a flattened summary, so we keep the recent raw media messages
// in a small bounded in-memory cache, keyed by user id + message id, so the API
// can download them on demand.

const mediaCachePerUser = 250

type cachedMedia struct {
	msg       *waProto.Message
	mediaType string
	mimetype  string
	filename  string
	caption   string
}

var (
	mediaCache     = map[string]map[string]*cachedMedia{}
	mediaCacheKeys = map[string][]string{}
	mediaCacheMu   sync.Mutex
)

// unwrapMessage peels off the ephemeral / view-once / device-sent / edited
// wrappers WhatsApp puts around the real content, so callers see the inner
// message (text, media, ...) instead of an empty outer envelope.
func unwrapMessage(msg *waProto.Message) *waProto.Message {
	for i := 0; msg != nil && i < 5; i++ {
		switch {
		case msg.GetEphemeralMessage().GetMessage() != nil:
			msg = msg.GetEphemeralMessage().GetMessage()
		case msg.GetViewOnceMessage().GetMessage() != nil:
			msg = msg.GetViewOnceMessage().GetMessage()
		case msg.GetViewOnceMessageV2().GetMessage() != nil:
			msg = msg.GetViewOnceMessageV2().GetMessage()
		case msg.GetViewOnceMessageV2Extension().GetMessage() != nil:
			msg = msg.GetViewOnceMessageV2Extension().GetMessage()
		case msg.GetDocumentWithCaptionMessage().GetMessage() != nil:
			msg = msg.GetDocumentWithCaptionMessage().GetMessage()
		case msg.GetDeviceSentMessage().GetMessage() != nil:
			msg = msg.GetDeviceSentMessage().GetMessage()
		case msg.GetEditedMessage().GetMessage() != nil:
			msg = msg.GetEditedMessage().GetMessage()
		default:
			return msg
		}
	}
	return msg
}

// extractBody returns the human-readable text of a (already unwrapped) message,
// covering plain text and extended text (link previews / replies). Returns ""
// when the message carries no text.
func extractBody(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}
	if c := msg.GetConversation(); c != "" {
		return c
	}
	if e := msg.GetExtendedTextMessage(); e != nil {
		if t := e.GetText(); t != "" {
			return t
		}
	}
	return ""
}

// mediaInfo inspects a message and, if it carries a downloadable media part,
// returns a descriptor for it.
func mediaInfo(msg *waProto.Message, id string) (*cachedMedia, bool) {
	if msg == nil {
		return nil, false
	}
	switch {
	case msg.GetImageMessage() != nil:
		m := msg.GetImageMessage()
		return &cachedMedia{msg: msg, mediaType: "image", mimetype: m.GetMimetype(), filename: filenameFor(id, m.GetMimetype(), "jpg"), caption: m.GetCaption()}, true
	case msg.GetVideoMessage() != nil:
		m := msg.GetVideoMessage()
		return &cachedMedia{msg: msg, mediaType: "video", mimetype: m.GetMimetype(), filename: filenameFor(id, m.GetMimetype(), "mp4"), caption: m.GetCaption()}, true
	case msg.GetAudioMessage() != nil:
		m := msg.GetAudioMessage()
		return &cachedMedia{msg: msg, mediaType: "audio", mimetype: m.GetMimetype(), filename: filenameFor(id, m.GetMimetype(), "ogg"), caption: ""}, true
	case msg.GetDocumentMessage() != nil:
		m := msg.GetDocumentMessage()
		name := m.GetFileName()
		if name == "" {
			name = filenameFor(id, m.GetMimetype(), "bin")
		}
		return &cachedMedia{msg: msg, mediaType: "document", mimetype: m.GetMimetype(), filename: name, caption: m.GetCaption()}, true
	case msg.GetStickerMessage() != nil:
		m := msg.GetStickerMessage()
		return &cachedMedia{msg: msg, mediaType: "sticker", mimetype: m.GetMimetype(), filename: filenameFor(id, m.GetMimetype(), "webp"), caption: ""}, true
	default:
		return nil, false
	}
}

func filenameFor(id, mime, fallbackExt string) string {
	ext := fallbackExt
	if i := strings.IndexByte(mime, '/'); i >= 0 {
		sub := cleanMime(mime[i+1:])
		if sub != "" && len(sub) <= 5 {
			ext = sub
		}
	}
	clean := strings.NewReplacer("/", "_", ":", "_", "@", "_", ".", "_").Replace(id)
	if clean == "" {
		clean = "media"
	}
	return clean + "." + ext
}

// cacheMedia stores a downloadable message, evicting the oldest entry once the
// per-user cap is exceeded.
func cacheMedia(userId, msgId string, cm *cachedMedia) {
	mediaCacheMu.Lock()
	defer mediaCacheMu.Unlock()

	if mediaCache[userId] == nil {
		mediaCache[userId] = map[string]*cachedMedia{}
	}
	if _, exists := mediaCache[userId][msgId]; !exists {
		mediaCacheKeys[userId] = append(mediaCacheKeys[userId], msgId)
	}
	mediaCache[userId][msgId] = cm

	keys := mediaCacheKeys[userId]
	for len(keys) > mediaCachePerUser {
		oldest := keys[0]
		keys = keys[1:]
		delete(mediaCache[userId], oldest)
	}
	mediaCacheKeys[userId] = keys
}

func getCachedMedia(userId, msgId string) *cachedMedia {
	mediaCacheMu.Lock()
	defer mediaCacheMu.Unlock()
	if m, ok := mediaCache[userId]; ok {
		return m[msgId]
	}
	return nil
}

// DownloadMessageMedia downloads the media for a previously received message and
// returns it as a base64 data URI together with its metadata.
func DownloadMessageMedia(userId, messageId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	if messageId == "" {
		return nil, fmt.Errorf("messageId is required")
	}
	cm := getCachedMedia(userId, messageId)
	if cm == nil {
		return nil, fmt.Errorf("no cached media for message %q (only recently received media can be downloaded)", messageId)
	}

	ctx, cancel := opCtx()
	defer cancel()
	data, err := cli.DownloadAny(ctx, cm.msg)
	if err != nil {
		return nil, err
	}

	mime := cm.mimetype
	if mime == "" {
		mime = "application/octet-stream"
	}
	dataURI := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	return map[string]interface{}{
		"messageId": messageId,
		"mediaType": cm.mediaType,
		"mimetype":  mime,
		"filename":  cm.filename,
		"caption":   cm.caption,
		"size":      len(data),
		"dataUri":   dataURI,
	}, nil
}
