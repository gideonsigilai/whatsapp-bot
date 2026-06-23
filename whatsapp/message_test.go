package whatsapp

import (
	"testing"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// Verifies that wrapped messages (ephemeral / device-sent / view-once) have
// their inner text extracted instead of falling back to "Media/Other Message".
func TestUnwrapAndExtractBody(t *testing.T) {
	inner := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String("hi from bot"),
		},
	}
	cases := []struct {
		name string
		msg  *waProto.Message
		want string
	}{
		{"plain", &waProto.Message{Conversation: proto.String("plain")}, "plain"},
		{"extended", inner, "hi from bot"},
		{"ephemeral", &waProto.Message{EphemeralMessage: &waProto.FutureProofMessage{Message: inner}}, "hi from bot"},
		{"deviceSent", &waProto.Message{DeviceSentMessage: &waProto.DeviceSentMessage{Message: inner}}, "hi from bot"},
		{"viewOnceV2", &waProto.Message{ViewOnceMessageV2: &waProto.FutureProofMessage{Message: &waProto.Message{Conversation: proto.String("vo")}}}, "vo"},
	}
	for _, c := range cases {
		if got := extractBody(unwrapMessage(c.msg)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
