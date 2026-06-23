package whatsapp

import (
	"strings"
	"testing"
)

// Verifies the SSRF dialer guard refuses internal / metadata targets.
func TestResolveMediaBlocksInternalHosts(t *testing.T) {
	for _, u := range []string{
		"http://169.254.169.254/latest/meta-data/", // cloud metadata
		"http://127.0.0.1:8080/",                   // loopback
		"http://10.0.0.5/",                         // private
		"http://[::1]/",                            // ipv6 loopback
	} {
		if _, _, err := resolveMedia(u); err == nil {
			t.Errorf("expected %s to be refused, got nil error", u)
		}
	}
}

func TestResolveMediaDataURI(t *testing.T) {
	data, mime, err := resolveMedia("data:text/plain;base64,aGk=") // "hi"
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hi" {
		t.Errorf("data = %q, want hi", data)
	}
	if mime != "text/plain" {
		t.Errorf("mime = %q, want text/plain", mime)
	}
}

func TestResolveMediaOversizeBase64Rejected(t *testing.T) {
	// decodedLen must exceed maxMediaBytes; build just over the cap.
	payload := strings.Repeat("A", (maxMediaBytes/3+2)*4)
	if _, _, err := resolveMedia(payload); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("expected size-limit error, got %v", err)
	}
}
