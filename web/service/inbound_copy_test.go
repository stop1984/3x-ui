package service

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/op/go-logging"
)

var inboundCopyLoggerOnce sync.Once

func setupInboundCopyDB(t *testing.T) {
	t.Helper()
	inboundCopyLoggerOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })

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

func seedInboundCopyInbound(t *testing.T, tag string, port int, clients []model.Client) int {
	t.Helper()
	if clients == nil {
		clients = []model.Client{}
	}
	settingsBytes, err := json.Marshal(map[string]any{
		"clients": clients,
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
		StreamSettings: `{"network":"tcp","security":"tls"}`,
		Settings:       string(settingsBytes),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound %s: %v", tag, err)
	}
	return inbound.Id
}

func inboundCopyClientByEmail(t *testing.T, inboundID int, email string) model.Client {
	t.Helper()
	var inbound model.Inbound
	if err := database.GetDB().First(&inbound, inboundID).Error; err != nil {
		t.Fatalf("load inbound %d: %v", inboundID, err)
	}
	svc := &InboundService{}
	clients, err := svc.GetClients(&inbound)
	if err != nil {
		t.Fatalf("GetClients(%d): %v", inboundID, err)
	}
	for _, client := range clients {
		if client.Email == email {
			return client
		}
	}
	t.Fatalf("client %s not found in inbound %d", email, inboundID)
	return model.Client{}
}

func TestCopyInboundClientsDoesNotMutateSourceSubIDOnTargetFailure(t *testing.T) {
	setupInboundCopyDB(t)

	sourceClient := model.Client{
		ID:         "0d7c4d7d-51c7-4939-b3cc-4c7472f3d257",
		Email:      "copy-source@example.com",
		SubID:      "",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	sourceID := seedInboundCopyInbound(t, "copy-source", 21443, []model.Client{sourceClient})
	targetID := seedInboundCopyInbound(t, "copy-target", 22443, nil)

	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), sourceID, []model.Client{sourceClient}); err != nil {
		t.Fatalf("SyncInbound source: %v", err)
	}

	var target model.Inbound
	if err := database.GetDB().First(&target, targetID).Error; err != nil {
		t.Fatalf("load target inbound: %v", err)
	}
	target.Settings = "{"
	if err := database.GetDB().Save(&target).Error; err != nil {
		t.Fatalf("corrupt target inbound: %v", err)
	}

	inboundSvc := &InboundService{}
	result, needRestart, err := inboundSvc.CopyInboundClients(targetID, sourceID, []string{sourceClient.Email}, "")
	if err == nil {
		t.Fatalf("CopyInboundClients unexpectedly succeeded: %+v", result)
	}
	if needRestart {
		t.Fatalf("needRestart = true, want false when target add never succeeded")
	}

	rec, err := clientSvc.GetRecordByEmail(nil, sourceClient.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.SubID != "" {
		t.Fatalf("source client record subId = %q, want unchanged empty value", rec.SubID)
	}

	live := inboundCopyClientByEmail(t, sourceID, sourceClient.Email)
	if live.SubID != "" {
		t.Fatalf("source inbound client subId = %q, want unchanged empty value", live.SubID)
	}
}
