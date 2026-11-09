package service

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot"
	"github.com/lib/pq"
)

func newQQBotBindTestService(t *testing.T) (*QQBotService, sqlmock.Sqlmock, time.Time) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s, _ := newQQBotTestService(true)
	s.db = db
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	return s, mock, now
}

func TestQQBotBindCodeHelpers(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := newQQBotBindCode()
		if err != nil {
			t.Fatal(err)
		}
		if !validQQBotBindCode(c) {
			t.Fatalf("generated invalid code %q", c)
		}
		seen[c] = true
	}
	if len(seen) < 195 {
		t.Fatalf("codes are not random enough: %d unique of 200", len(seen))
	}
	if got := normalizeQQBotBindCode(" abcd-2345 "); got != "ABCD2345" {
		t.Fatalf("normalize = %q", got)
	}
	for _, bad := range []string{"", "ABCD234", "ABCD23456", "ABCD234O", "a@b.com", "ABCD2341"} {
		if validQQBotBindCode(normalizeQQBotBindCode(bad)) {
			t.Fatalf("%q should be invalid", bad)
		}
	}
	if qqbotBindCodeHash("A", "X") == qqbotBindCodeHash("B", "X") {
		t.Fatal("hash must be scoped to the app")
	}
}

func TestQQBotCreateBindCode(t *testing.T) {
	s, mock, now := newQQBotBindTestService(t)
	mock.ExpectQuery("SELECT COUNT").WithArgs("11111111", int64(7)).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM qqbot_bind_codes").WithArgs(int64(7), now).WillReturnResult(sqlmock.NewResult(0, 1))
	var storedHash string
	mock.ExpectExec("INSERT INTO qqbot_bind_codes").
		WithArgs("11111111", int64(7), hashCapture{&storedHash}, now.Add(qqbotBindCodeTTL), now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	code, err := s.CreateBindCode(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if !validQQBotBindCode(code.Code) || !code.ExpiresAt.Equal(now.Add(qqbotBindCodeTTL)) {
		t.Fatalf("unexpected code %+v", code)
	}
	if storedHash != qqbotBindCodeHash("11111111", code.Code) || strings.Contains(storedHash, code.Code) {
		t.Fatal("only the hash of the code may be stored")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQQBotCreateBindCodeAlreadyBound(t *testing.T) {
	s, mock, _ := newQQBotBindTestService(t)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
	if _, err := s.CreateBindCode(context.Background(), 7); !errors.Is(err, ErrQQBotAlreadyBound) {
		t.Fatalf("want ErrQQBotAlreadyBound, got %v", err)
	}
}

func TestQQBotCreateBindCodeRateLimited(t *testing.T) {
	s, mock, _ := newQQBotBindTestService(t)
	for i := 0; i < qqbotBindLimitPerMinute; i++ {
		mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
		_, _ = s.CreateBindCode(context.Background(), 7)
	}
	if _, err := s.CreateBindCode(context.Background(), 7); !errors.Is(err, ErrQQBotTooFrequent) {
		t.Fatalf("want ErrQQBotTooFrequent, got %v", err)
	}
}

func qqbotGroupMsg(content string) *qqbot.Message {
	return &qqbot.Message{Scene: qqbot.SceneGroup, MsgID: "m1", GroupOpenID: "G1", UserOpenID: "U1", Content: content, Addressed: true}
}

func TestQQBotCmdBindWithCode(t *testing.T) {
	s, mock, now := newQQBotBindTestService(t)
	cfg, _ := s.loadConfig(context.Background())
	mock.ExpectQuery("SELECT user_id FROM qqbot_bindings").WithArgs("11111111", "U1").WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery("DELETE FROM qqbot_bind_codes").
		WithArgs("11111111", qqbotBindCodeHash("11111111", "ABCD2345"), now).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(7)))
	mock.ExpectExec("INSERT INTO qqbot_bindings").WithArgs("11111111", "U1", int64(7), "group", now).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	r := s.HandleCommand(context.Background(), cfg, qqbotGroupMsg("/绑定 abcd-2345"))
	if r.Text != "绑定成功！现在可以发送 /签到 了" {
		t.Fatalf("unexpected reply %q", r.Text)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQQBotCmdBindRejects(t *testing.T) {
	s, mock, _ := newQQBotBindTestService(t)
	cfg, _ := s.loadConfig(context.Background())

	// Emails are no longer accepted and never reach the database.
	if r := s.HandleCommand(context.Background(), cfg, qqbotGroupMsg("/绑定 victim@example.com")); r.Text != qqbotBindUsage {
		t.Fatalf("email bind should show usage, got %q", r.Text)
	}

	// Unknown / expired / used code.
	mock.ExpectQuery("SELECT user_id FROM qqbot_bindings").WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery("DELETE FROM qqbot_bind_codes").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	if r := s.HandleCommand(context.Background(), cfg, qqbotGroupMsg("/绑定 ABCD2345")); !strings.Contains(r.Text, "无效或已过期") {
		t.Fatalf("unexpected reply %q", r.Text)
	}

	// Account already bound to another QQ (race): rolled back with a clear message.
	mock.ExpectQuery("SELECT user_id FROM qqbot_bindings").WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery("DELETE FROM qqbot_bind_codes").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(7)))
	mock.ExpectExec("INSERT INTO qqbot_bindings").WillReturnError(&pq.Error{Code: "23505"})
	mock.ExpectRollback()
	if r := s.HandleCommand(context.Background(), cfg, qqbotGroupMsg("/绑定 ABCD2345")); !strings.Contains(r.Text, "已绑定其他 QQ") {
		t.Fatalf("unexpected reply %q", r.Text)
	}

	// This QQ is already bound.
	mock.ExpectQuery("SELECT user_id FROM qqbot_bindings").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(9)))
	if r := s.HandleCommand(context.Background(), cfg, qqbotGroupMsg("/绑定 ABCD2345")); !strings.Contains(r.Text, "/解绑") {
		t.Fatalf("unexpected reply %q", r.Text)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	// Per-QQ rate limit stops code guessing.
	for i := 0; i < qqbotBindLimitPerMinute; i++ {
		s.limiter.incr("U1")
	}
	if r := s.HandleCommand(context.Background(), cfg, qqbotGroupMsg("/绑定 ABCD2345")); !strings.Contains(r.Text, "频繁") {
		t.Fatalf("unexpected reply %q", r.Text)
	}
}

func TestQQBotWebhookFullGroupMessages(t *testing.T) {
	s, sent := newQQBotTestService(true)
	ts := "1725442341"
	post := func(id, content string, extra ...string) {
		body := []byte(`{"id":"` + id + `","op":0,"t":"GROUP_MESSAGE_CREATE","d":{"id":"m-` + id + `","content":"` + content + `","group_openid":"G1","author":{"member_openid":"U1"}` + strings.Join(extra, "") + `}}`)
		if _, err := s.HandleWebhook(context.Background(), qqbotTestSign(ts, body), ts, body); err != nil {
			t.Fatal(err)
		}
	}

	post("e1", "大家好")
	post("e2", "/不存在的指令")
	post("e4", "/帮助", `,"author":{"id":"B1","bot":true}`) // own echo
	select {
	case r := <-sent:
		t.Fatalf("non-command chatter must be ignored, got %q", r.Text)
	case <-time.After(200 * time.Millisecond):
	}

	post("e5", "<@!B1> 帮助", `,"mentions":[{"is_you":true}]`)
	select {
	case r := <-sent:
		if !strings.Contains(r.Text, "/签到") {
			t.Fatalf("unexpected help reply %q", r.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("@bot without slash in full mode was not handled")
	}

	post("e3", "/帮助")
	select {
	case r := <-sent:
		if !strings.Contains(r.Text, "/签到") {
			t.Fatalf("unexpected help reply %q", r.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slash command without @ was not handled")
	}
}

// hashCapture matches any string argument and records it.
type hashCapture struct{ dst *string }

func (h hashCapture) Match(v driver.Value) bool {
	s, ok := v.(string)
	if ok {
		*h.dst = s
	}
	return ok && len(s) == 64
}
