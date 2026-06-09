package service

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/xray"
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

func TestAttachRollsBackWhenLaterInboundFails(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "74ba2a42-5f2a-4642-8fc9-3b16d994d92f",
		Email:      "attach-rollback@example.com",
		SubID:      "sub-attach-rollback",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Comment:    "stable",
	}
	ib1 := seedClientMutationInbound(t, "vless-attach-a", 16443, client)
	ib2 := seedClientMutationInboundClients(t, "vless-attach-b", 17443, nil)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}

	needRestart, err := clientSvc.Attach(inboundSvc, rec.Id, []int{ib2, 999999})
	if err == nil {
		t.Fatalf("Attach unexpectedly succeeded")
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime is absent")
	}

	attached, err := clientSvc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(attached) != 1 || attached[0] != ib1 {
		t.Fatalf("attached inbounds after rollback = %v, want [%d]", attached, ib1)
	}
	if inboundHasClientEmail(t, ib2, client.Email) {
		t.Fatalf("rollback left client attached to inbound %d", ib2)
	}
}

func TestDetachRollsBackWhenLaterInboundFails(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "a0e3d067-f242-45d0-b1f0-a3d55d1fb2f4",
		Email:      "detach-rollback@example.com",
		SubID:      "sub-detach-rollback",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Comment:    "stable",
	}
	ib1 := seedClientMutationInbound(t, "vless-detach-a", 18443, client)
	ib2 := seedClientMutationInbound(t, "vless-detach-b", 19443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib2).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib2, err)
	}
	corrupt.Settings = `{"clients":[]}`
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib2, err)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}

	needRestart, err := clientSvc.Detach(inboundSvc, rec.Id, []int{ib1, ib2})
	if err == nil {
		t.Fatalf("Detach unexpectedly succeeded")
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime is absent")
	}

	attached, err := clientSvc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(attached) != 2 {
		t.Fatalf("attached inbounds after rollback = %v, want both original inbounds", attached)
	}
	if !inboundHasClientEmail(t, ib1, client.Email) {
		t.Fatalf("rollback did not restore client on inbound %d", ib1)
	}
}

func TestCreateRollsBackWhenLaterInboundFails(t *testing.T) {
	setupClientMutationDB(t)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}

	ib1 := seedClientMutationInboundClients(t, "vless-create-a", 20443, nil)

	client := model.Client{
		Email:      "create-rollback@example.com",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Comment:    "new",
	}

	needRestart, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{
		Client:     client,
		InboundIds: []int{ib1, 999999},
	})
	if err == nil {
		t.Fatalf("Create unexpectedly succeeded")
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime is absent")
	}

	if inboundHasClientEmail(t, ib1, client.Email) {
		t.Fatalf("rollback left created client attached to inbound %d", ib1)
	}
	if _, err := clientSvc.GetRecordByEmail(nil, client.Email); err == nil {
		t.Fatalf("rollback left orphan client record for %s", client.Email)
	}
}

func TestDeleteRollsBackWhenLaterInboundFails(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "9b99f2b8-7a2e-4a75-a0c8-e69d77317f65",
		Email:      "delete-rollback@example.com",
		SubID:      "sub-delete-rollback",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Comment:    "stable",
	}
	ib1 := seedClientMutationInbound(t, "vless-delete-a", 21443, client)
	ib2 := seedClientMutationInbound(t, "vless-delete-b", 22443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib2).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib2, err)
	}
	corrupt.Settings = `{"clients":[]}`
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib2, err)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}

	needRestart, err := clientSvc.Delete(inboundSvc, rec.Id, false)
	if err == nil {
		t.Fatalf("Delete unexpectedly succeeded")
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime is absent")
	}

	kept, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail after rollback: %v", err)
	}
	attached, err := clientSvc.GetInboundIdsForRecord(kept.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(attached) != 2 {
		t.Fatalf("attached inbounds after rollback = %v, want both original inbounds", attached)
	}
	if !inboundHasClientEmail(t, ib1, client.Email) {
		t.Fatalf("rollback did not restore client on inbound %d", ib1)
	}
	if isClientEmailTombstoned(client.Email) {
		t.Fatalf("rollback left tombstone for restored client %s", client.Email)
	}
}

func TestResetClientTrafficUsesRequestedInboundForMultiAttachClient(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "bf3abff5-2081-45d7-976d-f45b0cebc660",
		Email:      "reset-multiattach@example.com",
		SubID:      "sub-reset-multiattach",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-reset-a", 23443, client)
	ib2 := seedClientMutationInbound(t, "vless-reset-b", 24443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	traffic := &xray.ClientTraffic{
		InboundId: ib1,
		Email:     client.Email,
		Enable:    false,
		Up:        123,
		Down:      456,
	}
	if err := database.GetDB().Create(traffic).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib1).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib1, err)
	}
	corrupt.Settings = `{"clients":[]}`
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib1, err)
	}

	needRestart, err := inboundSvc.ResetClientTraffic(ib2, client.Email)
	if err != nil {
		t.Fatalf("ResetClientTraffic(%d): %v", ib2, err)
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime is absent")
	}

	var updated xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", client.Email).First(&updated).Error; err != nil {
		t.Fatalf("reload traffic: %v", err)
	}
	if !updated.Enable {
		t.Fatalf("traffic enable remained false")
	}
	if updated.Up != 0 || updated.Down != 0 {
		t.Fatalf("traffic counters = up:%d down:%d, want 0/0", updated.Up, updated.Down)
	}
}

func TestResetTrafficByEmailPrevalidatesAllAttachedInbounds(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "1ddf9cd7-8487-4d7b-a712-faa8d8a6b028",
		Email:      "reset-prevalidate@example.com",
		SubID:      "sub-reset-prevalidate",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-reset-pre-a", 25443, client)
	ib2 := seedClientMutationInbound(t, "vless-reset-pre-b", 26443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	traffic := &xray.ClientTraffic{
		InboundId: ib1,
		Email:     client.Email,
		Enable:    false,
		Up:        321,
		Down:      654,
	}
	if err := database.GetDB().Create(traffic).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib2).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib2, err)
	}
	corrupt.Settings = `{"clients":[]}`
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib2, err)
	}

	needRestart, err := clientSvc.ResetTrafficByEmail(inboundSvc, client.Email)
	if err == nil {
		t.Fatalf("ResetTrafficByEmail unexpectedly succeeded")
	}
	if needRestart {
		t.Fatalf("needRestart = true, want false when prevalidation fails before reset")
	}

	var kept xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", client.Email).First(&kept).Error; err != nil {
		t.Fatalf("reload traffic: %v", err)
	}
	if kept.Enable {
		t.Fatalf("traffic enable changed on failed prevalidation")
	}
	if kept.Up != 321 || kept.Down != 654 {
		t.Fatalf("traffic counters changed on failed prevalidation: up:%d down:%d", kept.Up, kept.Down)
	}
}

func TestGetClientByEmailSkipsStaleTrafficOwnerInbound(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "8f0b9f3f-d0a4-4b45-bb8a-58e9ae8494cf",
		Email:      "lookup-shared@example.com",
		SubID:      "sub-lookup-shared",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-lookup-a", 27443, client)
	ib2 := seedClientMutationInbound(t, "vless-lookup-b", 28443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	traffic := &xray.ClientTraffic{
		InboundId: ib1,
		Email:     client.Email,
		Enable:    true,
		Up:        11,
		Down:      22,
	}
	if err := database.GetDB().Create(traffic).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	var stale model.Inbound
	if err := database.GetDB().First(&stale, ib1).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib1, err)
	}
	stale.Settings = `{"clients":[]}`
	if err := database.GetDB().Save(&stale).Error; err != nil {
		t.Fatalf("stale inbound %d: %v", ib1, err)
	}

	gotTraffic, gotClient, err := inboundSvc.GetClientByEmail(client.Email)
	if err != nil {
		t.Fatalf("GetClientByEmail: %v", err)
	}
	if gotTraffic == nil || gotClient == nil {
		t.Fatalf("GetClientByEmail returned nil traffic/client")
	}
	if gotTraffic.Email != client.Email {
		t.Fatalf("traffic email = %q, want %q", gotTraffic.Email, client.Email)
	}
	if gotClient.Email != client.Email {
		t.Fatalf("client email = %q, want %q", gotClient.Email, client.Email)
	}
	if gotClient.SubID != client.SubID {
		t.Fatalf("client subId = %q, want %q", gotClient.SubID, client.SubID)
	}
}
