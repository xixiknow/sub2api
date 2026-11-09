package qqbot

import (
	"encoding/json"
	"strings"
	"sync/atomic"
)

// Opcodes used in webhook mode.
const (
	OpDispatch        = 0
	OpHTTPCallbackAck = 12
	OpValidation      = 13
)

// Event types handled by the bot.
const (
	EventGroupAtMessage = "GROUP_AT_MESSAGE_CREATE"
	// EventGroupMessage is every group message, delivered only when the bot
	// has the "group full message" permission on the QQ open platform.
	EventGroupMessage = "GROUP_MESSAGE_CREATE"
	EventC2CMessage   = "C2C_MESSAGE_CREATE"
)

// Scene is where a message came from (and where the reply goes).
type Scene string

const (
	SceneGroup Scene = "group"
	SceneC2C   Scene = "c2c"
)

// Payload is the common webhook envelope.
type Payload struct {
	ID string          `json:"id"`
	Op int             `json:"op"`
	S  int64           `json:"s"`
	T  string          `json:"t"`
	D  json.RawMessage `json:"d"`
}

// ValidationRequest is the op=13 payload data.
type ValidationRequest struct {
	PlainToken string `json:"plain_token"`
	EventTs    string `json:"event_ts"`
}

// ValidationResponse is the op=13 reply body.
type ValidationResponse struct {
	PlainToken string `json:"plain_token"`
	Signature  string `json:"signature"`
}

type messageEventData struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	GroupOpenID string `json:"group_openid"`
	Author      struct {
		ID           string `json:"id"`
		UserOpenID   string `json:"user_openid"`
		MemberOpenID string `json:"member_openid"`
		UnionOpenID  string `json:"union_openid"`
		Bot          bool   `json:"bot"`
	} `json:"author"`
	// Mentions lists the @-mentions in a group message; is_you marks the bot itself.
	// In full-message mode @bot messages arrive as GROUP_MESSAGE_CREATE, so this is
	// the only way to tell them apart from ordinary chatter.
	Mentions []struct {
		IsYou bool `json:"is_you"`
	} `json:"mentions"`
}

// Message is a normalized incoming user message.
type Message struct {
	Scene       Scene
	MsgID       string // used for passive replies
	GroupOpenID string // set for SceneGroup
	UserOpenID  string // sender openid (member_openid in groups, user_openid in C2C)
	Content     string // trimmed text, @mention removed
	// Addressed is true when the user clearly spoke to the bot: a C2C message
	// or a group message that @-mentions it. Plain group messages are false.
	Addressed bool
	// FromBot is set for messages sent by a bot (including this bot's own echo
	// in full-message mode); they must never be answered.
	FromBot bool

	seq atomic.Int32
}

// NextSeq returns the msg_seq for the next passive reply to this message
// (QQ rejects duplicate seq values for the same msg_id).
func (m *Message) NextSeq() int { return int(m.seq.Add(1)) }

// ReplyTarget is the openid the reply is addressed to.
func (m *Message) ReplyTarget() string {
	if m.Scene == SceneGroup {
		return m.GroupOpenID
	}
	return m.UserOpenID
}

// ParseMessage normalizes GROUP_AT_MESSAGE_CREATE / GROUP_MESSAGE_CREATE /
// C2C_MESSAGE_CREATE events.
// ok is false for other event types.
func ParseMessage(p *Payload) (*Message, bool, error) {
	var scene Scene
	addressed := true
	switch p.T {
	case EventGroupAtMessage:
		scene = SceneGroup
	case EventGroupMessage:
		scene, addressed = SceneGroup, false
	case EventC2CMessage:
		scene = SceneC2C
	default:
		return nil, false, nil
	}
	var d messageEventData
	if err := json.Unmarshal(p.D, &d); err != nil {
		return nil, false, err
	}
	if !addressed {
		for _, mt := range d.Mentions {
			if mt.IsYou {
				addressed = true
				break
			}
		}
	}
	m := &Message{
		Scene: scene, MsgID: d.ID, GroupOpenID: d.GroupOpenID, Content: cleanContent(d.Content),
		Addressed: addressed, FromBot: d.Author.Bot,
	}
	for _, id := range []string{d.Author.MemberOpenID, d.Author.UserOpenID, d.Author.ID} {
		if id != "" {
			m.UserOpenID = id
			break
		}
	}
	return m, true, nil
}

// cleanContent drops <@!xxx> mentions and surrounding whitespace.
func cleanContent(s string) string {
	for {
		i := strings.Index(s, "<@")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], ">")
		if j < 0 {
			break
		}
		s = s[:i] + s[i+j+1:]
	}
	return strings.TrimSpace(strings.ReplaceAll(s, " ", " "))
}
