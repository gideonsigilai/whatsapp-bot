package whatsapp

import (
	"sort"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

// ── WhatsApp Status (Stories) feed ──
//
// Status updates are messages posted to the status@broadcast JID. They must not
// land in the normal inbox, so the event handler routes them here into a small
// bounded per-user feed. Media items are also cached (via the media cache) so
// they can be downloaded with /api/download-media like any other message.

const statusPerUser = 300

type statusUpdate struct {
	poster     string
	posterName string
	id         string
	body       string
	hasMedia   bool
	mediaType  string
	mediaMime  string
	timestamp  time.Time
}

var (
	statusStore = map[string][]statusUpdate{}
	statusMu    sync.Mutex
)

// recordStatusUpdate ingests a status@broadcast message into the status feed.
func recordStatusUpdate(userId string, v *events.Message) {
	poster := v.Info.Sender.ToNonAD().String()
	name := v.Info.PushName
	if name == "" {
		name = v.Info.Sender.User
	}

	su := statusUpdate{
		poster:     poster,
		posterName: name,
		id:         v.Info.ID,
		timestamp:  v.Info.Timestamp.UTC(),
	}

	inner := unwrapMessage(v.Message)
	if cm, ok := mediaInfo(inner, v.Info.ID); ok {
		cacheMedia(userId, v.Info.ID, cm)
		su.hasMedia = true
		su.mediaType = cm.mediaType
		su.mediaMime = cm.mimetype
		su.body = cm.caption
	} else {
		su.body = extractBody(inner)
	}

	statusMu.Lock()
	list := append(statusStore[userId], su)
	if len(list) > statusPerUser {
		list = list[len(list)-statusPerUser:]
	}
	statusStore[userId] = list
	statusMu.Unlock()

	Broadcast(userId, "status_update", map[string]interface{}{
		"poster":    su.poster,
		"name":      su.posterName,
		"id":        su.id,
		"body":      su.body,
		"hasMedia":  su.hasMedia,
		"mediaType": su.mediaType,
		"timestamp": su.timestamp.Format(time.RFC3339),
	})
}

// GetStatusUpdates returns the captured status posts grouped by poster, most
// recently active poster first, each poster's items oldest-first.
func GetStatusUpdates(userId string) (interface{}, error) {
	statusMu.Lock()
	items := make([]statusUpdate, len(statusStore[userId]))
	copy(items, statusStore[userId])
	statusMu.Unlock()

	type group struct {
		poster string
		name   string
		latest time.Time
		items  []map[string]interface{}
	}
	groups := map[string]*group{}
	order := []string{}
	for _, su := range items {
		g := groups[su.poster]
		if g == nil {
			g = &group{poster: su.poster, name: su.posterName}
			groups[su.poster] = g
			order = append(order, su.poster)
		}
		if su.posterName != "" {
			g.name = su.posterName
		}
		if su.timestamp.After(g.latest) {
			g.latest = su.timestamp
		}
		g.items = append(g.items, map[string]interface{}{
			"id":        su.id,
			"body":      su.body,
			"hasMedia":  su.hasMedia,
			"mediaType": su.mediaType,
			"mediaMime": su.mediaMime,
			"timestamp": su.timestamp.Format(time.RFC3339),
		})
	}

	out := make([]map[string]interface{}, 0, len(order))
	for _, p := range order {
		g := groups[p]
		out = append(out, map[string]interface{}{
			"poster": g.poster,
			"name":   g.name,
			"count":  len(g.items),
			"latest": g.latest.Format(time.RFC3339),
			"items":  g.items,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["latest"].(string) > out[j]["latest"].(string)
	})
	return out, nil
}
