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

var clientMutationLoggerOnce sync.Once

func setupClientMutationDB(t *testing.T) {
	t.Helper()
	clientMutationLoggerOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })

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

func seedClientMutationInboundClients(t *testing.T, tag string, port int, clients []model.Client) int {
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
		t.Fatalf("seed inbound %s: %v", tag, err)
	}
	return inbound.Id
}

func seedClientMutationInbound(t *testing.T, tag string, port int, client model.Client) int {
	t.Helper()
	return seedClientMutationInboundClients(t, tag, port, []model.Client{client})
}

func inboundClientByEmail(t *testing.T, inboundID int, email string) model.Client {
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

func inboundHasClientEmail(t *testing.T, inboundID int, email string) bool {
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
			return true
		}
	}
	return false
}

func TestSetClientEnableByEmailUpdatesAllAttachedInbounds(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "2e0d13b1-196d-4b57-a3c1-9b27bc9f3491",
		Email:      "multi@example.com",
		SubID:      "sub-multi",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-a", 10443, client)
	ib2 := seedClientMutationInbound(t, "vless-b", 11443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	enabled, needRestart, err := clientSvc.SetClientEnableByEmail(inboundSvc, client.Email, false)
	if err != nil {
		t.Fatalf("SetClientEnableByEmail: %v", err)
	}
	if enabled {
		t.Fatalf("enabled = true, want false")
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime is absent")
	}

	c1 := inboundClientByEmail(t, ib1, client.Email)
	c2 := inboundClientByEmail(t, ib2, client.Email)
	if c1.Enable {
		t.Fatalf("inbound %d client remained enabled", ib1)
	}
	if c2.Enable {
		t.Fatalf("inbound %d client remained enabled", ib2)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.Enable {
		t.Fatalf("client record remained enabled")
	}
}

func TestResetClientIpLimitByEmailUpdatesAllAttachedInbounds(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "37c808b8-d5f9-49bd-8d3b-e715f5d7c733",
		Email:      "limit@example.com",
		SubID:      "sub-limit",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-c", 12443, client)
	ib2 := seedClientMutationInbound(t, "vless-d", 13443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	needRestart, err := clientSvc.ResetClientIpLimitByEmail(inboundSvc, client.Email, 7)
	if err != nil {
		t.Fatalf("ResetClientIpLimitByEmail: %v", err)
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime is absent")
	}

	c1 := inboundClientByEmail(t, ib1, client.Email)
	c2 := inboundClientByEmail(t, ib2, client.Email)
	if c1.LimitIP != 7 {
		t.Fatalf("inbound %d limitIp = %d, want 7", ib1, c1.LimitIP)
	}
	if c2.LimitIP != 7 {
		t.Fatalf("inbound %d limitIp = %d, want 7", ib2, c2.LimitIP)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.LimitIP != 7 {
		t.Fatalf("client record limitIp = %d, want 7", rec.LimitIP)
	}
}

func TestSaveByEmailRollsBackWhenAttachmentSyncFails(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "179700c5-f7eb-468d-91da-a8414f9565dd",
		Email:      "rollback@example.com",
		SubID:      "sub-rollback",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Comment:    "before",
	}
	ib1 := seedClientMutationInbound(t, "vless-rb-a", 14443, client)
	ib2 := seedClientMutationInboundClients(t, "vless-rb-b", 15443, nil)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}

	updated := client
	updated.Comment = "after"

	needRestart, err := clientSvc.SaveByEmail(inboundSvc, client.Email, &ClientCreatePayload{
		Client:     updated,
		InboundIds: []int{ib1, ib2, 999999},
	})
	if err == nil {
		t.Fatalf("SaveByEmail unexpectedly succeeded")
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime is absent")
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.Comment != "before" {
		t.Fatalf("client record comment = %q, want rollback to %q", rec.Comment, "before")
	}

	attached, err := clientSvc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(attached) != 1 || attached[0] != ib1 {
		t.Fatalf("attached inbounds after rollback = %v, want [%d]", attached, ib1)
	}

	c1 := inboundClientByEmail(t, ib1, client.Email)
	if c1.Comment != "before" {
		t.Fatalf("inbound %d comment = %q, want rollback to %q", ib1, c1.Comment, "before")
	}
	if inboundHasClientEmail(t, ib2, client.Email) {
		t.Fatalf("rollback left client attached to inbound %d", ib2)
	}
}
