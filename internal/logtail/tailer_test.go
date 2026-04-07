package logtail

import (
	"strings"
	"testing"
)

// --- ParseLogLine tests ---

func TestParseLogLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantTS    string
		wantMod   string
		wantMsg   string
		wantField map[string]string // subset of fields to check
	}{
		{
			name:    "timestamp + module + JSON",
			line:    `2026-04-06T14:18:55.123Z submission: incoming message	{"msg_id":"abc123","sender":"csb@drcs.ca","src_ip":"172.21.0.1:58098","username":"csb"}`,
			wantTS:  "2026-04-06T14:18:55.123Z",
			wantMod: "submission",
			wantMsg: "incoming message",
			wantField: map[string]string{
				"msg_id":   "abc123",
				"sender":   "csb@drcs.ca",
				"src_ip":   "172.21.0.1:58098",
				"username": "csb",
			},
		},
		{
			name:    "no timestamp",
			line:    `queue: delivered	{"attempt":1,"msg_id":"abc123","rcpt":"d@drcs.ca"}`,
			wantTS:  "",
			wantMod: "queue",
			wantMsg: "delivered",
			wantField: map[string]string{
				"msg_id": "abc123",
				"rcpt":   "d@drcs.ca",
			},
		},
		{
			name:    "queue delivered line",
			line:    `queue: delivered	{"attempt":1,"msg_id":"xyz789","rcpt":"user@example.com"}`,
			wantTS:  "",
			wantMod: "queue",
			wantMsg: "delivered",
			wantField: map[string]string{
				"msg_id":  "xyz789",
				"rcpt":    "user@example.com",
				"attempt": "1",
			},
		},
		{
			name:    "RCPT ok line",
			line:    `2026-04-06T14:18:55.200Z submission: RCPT ok	{"msg_id":"abc123","rcpt":"d@drcs.ca"}`,
			wantTS:  "2026-04-06T14:18:55.200Z",
			wantMod: "submission",
			wantMsg: "RCPT ok",
			wantField: map[string]string{
				"msg_id": "abc123",
				"rcpt":   "d@drcs.ca",
			},
		},
		{
			name:    "RCPT error line",
			line:    `submission: RCPT error	{"msg_id":"abc123","rcpt":"d@drcs.ca","smtp_msg":"Sender domain not configured"}`,
			wantTS:  "",
			wantMod: "submission",
			wantMsg: "RCPT error",
			wantField: map[string]string{
				"msg_id":   "abc123",
				"rcpt":     "d@drcs.ca",
				"smtp_msg": "Sender domain not configured",
			},
		},
		{
			name:    "delivery attempt failed",
			line:    `queue: delivery attempt failed	{"msg_id":"abc123","reason":"connection refused"}`,
			wantTS:  "",
			wantMod: "queue",
			wantMsg: "delivery attempt failed",
			wantField: map[string]string{
				"msg_id": "abc123",
				"reason": "connection refused",
			},
		},
		{
			name:    "authentication failed",
			line:    `submission/sasl: authentication failed	{"username":"baduser","src_ip":"10.0.0.1:12345","reason":"invalid credentials"}`,
			wantTS:  "",
			wantMod: "submission/sasl",
			wantMsg: "authentication failed",
			wantField: map[string]string{
				"username": "baduser",
				"src_ip":   "10.0.0.1:12345",
				"reason":   "invalid credentials",
			},
		},
		{
			name:      "plain text line (no tab, no JSON)",
			line:      "loading configuration...",
			wantTS:    "",
			wantMod:   "",
			wantMsg:   "loading configuration...",
			wantField: nil,
		},
		{
			name:    "numeric JSON values converted to strings",
			line:    `queue: delivered	{"attempt":3,"msg_id":"n1","rcpt":"a@b.com"}`,
			wantTS:  "",
			wantMod: "queue",
			wantMsg: "delivered",
			wantField: map[string]string{
				"attempt": "3",
				"msg_id":  "n1",
				"rcpt":    "a@b.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLogLine(tt.line)

			if got.Timestamp != tt.wantTS {
				t.Errorf("Timestamp = %q, want %q", got.Timestamp, tt.wantTS)
			}
			if got.Module != tt.wantMod {
				t.Errorf("Module = %q, want %q", got.Module, tt.wantMod)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
			for k, want := range tt.wantField {
				if gotV, ok := got.Fields[k]; !ok {
					t.Errorf("Fields[%q] missing", k)
				} else if gotV != want {
					t.Errorf("Fields[%q] = %q, want %q", k, gotV, want)
				}
			}
			if tt.wantField == nil && got.Fields != nil {
				t.Errorf("Fields = %v, want nil", got.Fields)
			}
		})
	}
}

// --- EventToMailLog tests ---

func TestEventToMailLog(t *testing.T) {
	tests := []struct {
		name       string
		evt        LogEvent
		wantNil    bool
		wantStatus string
		wantFrom   string
		wantTo     string
		wantSrcIP  string
		wantPort   string
		wantErr    string // empty means nil error
		wantMsgPfx string // prefix of MsgID (for auth events)
	}{
		{
			name: "incoming message via submission",
			evt: LogEvent{
				Module:  "submission",
				Message: "incoming message",
				Fields: map[string]string{
					"msg_id": "msg1",
					"sender": "user@drcs.ca",
					"src_ip": "172.21.0.1:58098",
				},
			},
			wantStatus: "accepted",
			wantFrom:   "user@drcs.ca",
			wantSrcIP:  "172.21.0.1",
			wantPort:   "587",
		},
		{
			name: "incoming message via smtp (port 25)",
			evt: LogEvent{
				Module:  "smtp",
				Message: "incoming message",
				Fields: map[string]string{
					"msg_id": "msg2",
					"sender": "app@local",
					"src_ip": "10.0.0.5:9999",
				},
			},
			wantStatus: "accepted",
			wantFrom:   "app@local",
			wantSrcIP:  "10.0.0.5",
			wantPort:   "25",
		},
		{
			name: "RCPT ok",
			evt: LogEvent{
				Module:  "submission",
				Message: "RCPT ok",
				Fields: map[string]string{
					"msg_id": "msg1",
					"rcpt":   "recipient@example.com",
				},
			},
			wantStatus: "accepted",
			wantTo:     "recipient@example.com",
		},
		{
			name: "delivered",
			evt: LogEvent{
				Module:  "queue",
				Message: "delivered",
				Fields: map[string]string{
					"msg_id": "msg1",
					"rcpt":   "recipient@example.com",
				},
			},
			wantStatus: "sent",
			wantTo:     "recipient@example.com",
		},
		{
			name: "delivery attempt failed",
			evt: LogEvent{
				Module:  "queue",
				Message: "delivery attempt failed",
				Fields: map[string]string{
					"msg_id": "msg1",
					"reason": "connection refused",
				},
			},
			wantStatus: "deferred",
			wantErr:    "connection refused",
		},
		{
			name: "not delivered permanent error",
			evt: LogEvent{
				Module:  "queue",
				Message: "not delivered, permanent error",
				Fields: map[string]string{
					"msg_id": "msg1",
					"reason": "550 user unknown",
				},
			},
			wantStatus: "failed",
			wantErr:    "550 user unknown",
		},
		{
			name: "RCPT error",
			evt: LogEvent{
				Module:  "submission",
				Message: "RCPT error",
				Fields: map[string]string{
					"msg_id":   "msg1",
					"rcpt":     "bad@example.com",
					"smtp_msg": "Sender domain not configured",
				},
			},
			wantStatus: "rejected",
			wantTo:     "bad@example.com",
			wantErr:    "Sender domain not configured",
		},
		{
			name: "authentication failed",
			evt: LogEvent{
				Module:  "submission/sasl",
				Message: "authentication failed",
				Fields: map[string]string{
					"username": "baduser",
					"src_ip":   "10.0.0.1:12345",
					"reason":   "invalid credentials",
				},
			},
			wantStatus: "rejected",
			wantFrom:   "baduser",
			wantSrcIP:  "10.0.0.1",
			wantPort:   "587",
			wantErr:    "invalid credentials",
			wantMsgPfx: "auth-baduser-",
		},
		{
			name: "unknown event with msg_id",
			evt: LogEvent{
				Module:  "queue",
				Message: "some other event",
				Fields: map[string]string{
					"msg_id": "msg1",
				},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EventToMailLog(tt.evt)

			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected non-nil result")
			}

			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.FromAddr != tt.wantFrom {
				t.Errorf("FromAddr = %q, want %q", got.FromAddr, tt.wantFrom)
			}
			if got.ToAddr != tt.wantTo {
				t.Errorf("ToAddr = %q, want %q", got.ToAddr, tt.wantTo)
			}
			if got.SourceIP != tt.wantSrcIP {
				t.Errorf("SourceIP = %q, want %q", got.SourceIP, tt.wantSrcIP)
			}
			if got.SourcePort != tt.wantPort {
				t.Errorf("SourcePort = %q, want %q", got.SourcePort, tt.wantPort)
			}

			if tt.wantErr == "" {
				if got.Error != nil {
					t.Errorf("Error = %q, want nil", *got.Error)
				}
			} else {
				if got.Error == nil {
					t.Errorf("Error = nil, want %q", tt.wantErr)
				} else if *got.Error != tt.wantErr {
					t.Errorf("Error = %q, want %q", *got.Error, tt.wantErr)
				}
			}

			if tt.wantMsgPfx != "" {
				if !strings.HasPrefix(got.MsgID, tt.wantMsgPfx) {
					t.Errorf("MsgID = %q, want prefix %q", got.MsgID, tt.wantMsgPfx)
				}
			}
		})
	}
}

func TestEventToMailLogIgnored(t *testing.T) {
	ignored := []LogEvent{
		{Message: "loading configuration..."},
		{Module: "tls", Message: "certificate loaded"},
		{Module: "dns", Message: "looking up MX"},
		{Module: "queue", Message: "retry scheduled", Fields: map[string]string{}},
		{Message: "some random line"},
		// Event with no msg_id and not auth failure.
		{Module: "submission", Message: "connection established", Fields: map[string]string{
			"src_ip": "1.2.3.4:5678",
		}},
	}

	for _, evt := range ignored {
		t.Run(evt.Message, func(t *testing.T) {
			got := EventToMailLog(evt)
			if got != nil {
				t.Errorf("expected nil for %q, got %+v", evt.Message, got)
			}
		})
	}
}
