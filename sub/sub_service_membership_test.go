package sub

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/op/go-logging"
)

var subMembershipLoggerOnce sync.Once

func setupSubMembershipDB(t *testing.T) {
	t.Helper()
	subMembershipLoggerOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })

	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Logf("CloseDB warning: %v", err)
		}
	})
}

func seedSubMembershipInbound(t *testing.T, tag string, port int, client model.Client) *model.Inbound {
	t.Helper()
	settingsBytes, err := json.Marshal(map[string]any{
		"clients":    []model.Client{client},
		"decryption": "none",
	})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	inbound := &model.Inbound{
		Tag:            tag,
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           port,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"none"}`,
		Settings:       string(settingsBytes),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound %s: %v", tag, err)
	}
	return inbound
}

func TestGetSubsUsesCanonicalSubMembershipWhenInboundSubIDIsStale(t *testing.T) {
	setupSubMembershipDB(t)

	const (
		email = "member@example.com"
		subID = "sub-123"
	)

	inboundA := seedSubMembershipInbound(t, "sub-a", 443, model.Client{
		ID:     "uuid-a",
		Email:  email,
		SubID:  subID,
		Enable: true,
	})
	inboundB := seedSubMembershipInbound(t, "sub-b", 8443, model.Client{
		ID:     "uuid-b",
		Email:  email,
		SubID:  "",
		Enable: true,
	})

	rec := &model.ClientRecord{
		Email:     email,
		SubID:     subID,
		UUID:      "uuid-a",
		Enable:    true,
		TotalGB:   1024,
		LimitIP:   1,
		Comment:   "shared client",
		CreatedAt: 1,
		UpdatedAt: 1,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}
	links := []model.ClientInbound{
		{ClientId: rec.Id, InboundId: inboundA.Id, FlowOverride: ""},
		{ClientId: rec.Id, InboundId: inboundB.Id, FlowOverride: ""},
	}
	if err := database.GetDB().Create(&links).Error; err != nil {
		t.Fatalf("create client_inbounds: %v", err)
	}

	svc := NewSubService(false, "-ieo")
	subs, _, _, err := svc.GetSubs(subID, "stub.example")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("len(subs) = %d, want 2; subs=%v", len(subs), subs)
	}
	joined := strings.Join(subs, "\n")
	if !strings.Contains(joined, "vless://uuid-a@stub.example:443") {
		t.Fatalf("missing first inbound link in %q", joined)
	}
	if !strings.Contains(joined, "vless://uuid-b@stub.example:8443") {
		t.Fatalf("missing stale-subId inbound link in %q", joined)
	}
}
