package instagram

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testAppSecret = "test_app_secret"

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHandler_Verification(t *testing.T) {
	wh := NewWebhookHandler(testAppSecret, "test_verify_token")

	tests := []struct {
		name       string
		mode       string
		token      string
		challenge  string
		wantStatus int
	}{
		{"valid verification", "subscribe", "test_verify_token", "test_challenge_123", http.StatusOK},
		{"invalid mode", "unsubscribe", "test_verify_token", "test_challenge", http.StatusForbidden},
		{"invalid token", "subscribe", "wrong_token", "test_challenge", http.StatusForbidden},
		{"invalid challenge format", "subscribe", "test_verify_token", "<script>alert(1)</script>", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode="+tt.mode+"&hub.verify_token="+tt.token+"&hub.challenge="+tt.challenge, nil)
			w := httptest.NewRecorder()
			wh.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK && w.Body.String() != tt.challenge {
				t.Errorf("body = %q, want %q", w.Body.String(), tt.challenge)
			}
		})
	}
}

func TestWebhookHandler_MethodNotAllowed(t *testing.T) {
	wh := NewWebhookHandler(testAppSecret, "test_token")
	req := httptest.NewRequest(http.MethodPut, "/webhook", nil)
	w := httptest.NewRecorder()
	wh.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// validInstagramPayload is a realistic Meta-shaped Instagram Messaging webhook body.
const validInstagramPayload = `{
  "object": "instagram",
  "entry": [{
    "id": "17841400000000000",
    "time": 1733520000000,
    "messaging": [{
      "sender": {"id": "12334"},
      "recipient": {"id": "17841400000000000"},
      "timestamp": 1733520000000,
      "message": {"mid": "mid.msg_abc_123", "text": "hello"}
    }]
  }]
}`

func TestWebhookHandler_PostSignatureRequired(t *testing.T) {
	wh := NewWebhookHandler(testAppSecret, "vt")

	t.Run("missing signature dropped", func(t *testing.T) {
		got := captureEvent(t, wh, []byte(validInstagramPayload), "")
		if got != nil {
			t.Errorf("unsigned POST was accepted: %+v", got)
		}
	})

	t.Run("bad signature dropped", func(t *testing.T) {
		got := captureEvent(t, wh, []byte(validInstagramPayload), "sha256="+strings.Repeat("00", 32))
		if got != nil {
			t.Errorf("bad-signature POST was accepted")
		}
	})

	t.Run("valid signature accepted and parses sender/recipient", func(t *testing.T) {
		body := []byte(validInstagramPayload)
		got := captureEvent(t, wh, body, sign(body, testAppSecret))
		if got == nil {
			t.Fatal("valid POST was dropped")
		}
		if got.entry.ID != "17841400000000000" {
			t.Errorf("entry.ID = %q, want 17841400000000000", got.entry.ID)
		}
		if got.event.Sender.ID != "12334" {
			t.Errorf("sender.ID = %q (json tag regression?) want 12334", got.event.Sender.ID)
		}
		if got.event.Recipient.ID != "17841400000000000" {
			t.Errorf("recipient.ID = %q, want 17841400000000000", got.event.Recipient.ID)
		}
		if got.event.Message == nil || got.event.Message.ID != "mid.msg_abc_123" {
			t.Errorf("message.mid parse failed: %+v", got.event.Message)
		}
		if got.event.Message.Text != "hello" {
			t.Errorf("message.text = %q, want hello", got.event.Message.Text)
		}
	})

	t.Run("empty app secret drops even valid sig", func(t *testing.T) {
		empty := NewWebhookHandler("", "vt")
		body := []byte(validInstagramPayload)
		got := captureEvent(t, empty, body, sign(body, ""))
		if got != nil {
			t.Errorf("POST accepted when app_secret missing")
		}
	})

	t.Run("malformed JSON returns 200 and is dropped", func(t *testing.T) {
		body := []byte("{not json")
		got := captureEvent(t, wh, body, sign(body, testAppSecret))
		if got != nil {
			t.Errorf("malformed body dispatched to handler")
		}
	})

	t.Run("wrong object type dropped", func(t *testing.T) {
		body := []byte(`{"object":"page","entry":[]}`)
		got := captureEvent(t, wh, body, sign(body, testAppSecret))
		if got != nil {
			t.Errorf("non-instagram object dispatched")
		}
	})
}

type captured struct {
	entry WebhookEntry
	event MessagingEvent
}

func captureEvent(t *testing.T, wh *WebhookHandler, body []byte, sig string) *captured {
	t.Helper()
	var got *captured
	wh.onMessage = func(e WebhookEntry, ev MessagingEvent) {
		got = &captured{entry: e, event: ev}
	}
	defer func() { wh.onMessage = nil }()

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	if sig != "" {
		req.Header.Set("X-Hub-Signature-256", sig)
	}
	w := httptest.NewRecorder()
	wh.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("webhook POST status = %d, want 200 (always)", w.Code)
	}
	return got
}

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int // rune length
	}{
		{"short message", "Hello", 5},
		{"exactly max length", strings.Repeat("a", instagramMaxChars), instagramMaxChars},
		{"too-long ascii truncates to max", strings.Repeat("a", instagramMaxChars+100), instagramMaxChars},
		{"utf8 content preserves rune boundary", strings.Repeat("ñ", instagramMaxChars+50), instagramMaxChars},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMessage(tt.input)
			if n := len([]rune(got)); n != tt.wantLen {
				t.Errorf("rune len = %d, want %d", n, tt.wantLen)
			}
		})
	}
}

func TestFormatMessage_NoInvalidUtf8(t *testing.T) {
	// Pre-fix: byte slice of a multibyte rune produced invalid UTF-8.
	input := strings.Repeat("ñ", instagramMaxChars+10)
	got := FormatMessage(input)
	for i, r := range got {
		if r == '�' {
			t.Fatalf("FormatMessage produced replacement char at byte %d (invalid UTF-8)", i)
		}
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ellipsis suffix, got tail %q", got[max(0, len(got)-5):])
	}
}

func TestDedupKey_StableAcrossTimestamps(t *testing.T) {
	// Regression guard for Critical-4: string(rune(event.Timestamp)) collapsed all
	// large timestamps to U+FFFD, dropping every subsequent message from one sender
	// for 24h. message_handler now uses strconv.FormatInt for the fallback key and
	// event.Message.ID as the primary key. Verify both paths produce distinct keys.
	a := fmt.Sprintf("entry:sender:%d", int64(1733520000000))
	b := fmt.Sprintf("entry:sender:%d", int64(1733520000001))
	if a == b {
		t.Fatalf("distinct timestamps collapsed to same dedup key: %q", a)
	}
}
