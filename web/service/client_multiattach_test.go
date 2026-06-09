package service

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/xray"
	"github.com/op/go-logging"
)

var clientMutationLoggerOnce sync.Once

func setupClientMutationDB(t *testing.T) {
	t.Helper()
	clientMutationLoggerOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })
	prevManager := runtime.GetManager()
	runtime.SetManager(nil)

	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		runtime.SetManager(prevManager)
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

func TestUpdateRollsBackWhenLaterInboundFails(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "c913a6ae-faa6-4e59-9790-f3c92d85f6b3",
		Email:      "update-rollback@example.com",
		SubID:      "sub-update-rollback",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Comment:    "before",
	}
	ib1 := seedClientMutationInbound(t, "vless-update-a", 14543, client)
	ib2 := seedClientMutationInbound(t, "vless-update-b", 15543, client)

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
	corrupt.Settings = `{"clients":` // invalid JSON to force a later per-inbound update failure
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib2, err)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}

	updated := client
	updated.Email = "update-rollback-new@example.com"
	updated.Comment = "after"

	needRestart, err := clientSvc.Update(inboundSvc, rec.Id, updated)
	if err == nil {
		t.Fatalf("Update unexpectedly succeeded")
	}
	_ = needRestart

	rolledBack, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail rollback state: %v", err)
	}
	if rolledBack.Comment != "before" {
		t.Fatalf("client record comment = %q, want rollback to %q", rolledBack.Comment, "before")
	}
	if _, err := clientSvc.GetRecordByEmail(nil, updated.Email); err == nil {
		t.Fatalf("rollback left renamed client record %q behind", updated.Email)
	}

	c1 := inboundClientByEmail(t, ib1, client.Email)
	if c1.Comment != "before" {
		t.Fatalf("inbound %d comment = %q, want rollback to %q", ib1, c1.Comment, "before")
	}
	if inboundHasClientEmail(t, ib1, updated.Email) {
		t.Fatalf("rollback left renamed client on inbound %d", ib1)
	}
}

func TestUpdateRejectsPartialInboundFilterForSharedClient(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "9f97406e-5ac4-454b-b5c0-b41c511dd0ef",
		Email:      "update-filter@example.com",
		SubID:      "sub-update-filter",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Comment:    "before",
	}
	ib1 := seedClientMutationInbound(t, "vless-update-filter-a", 14643, client)
	ib2 := seedClientMutationInbound(t, "vless-update-filter-b", 15643, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}

	updated := client
	updated.Comment = "after"

	needRestart, err := clientSvc.Update(inboundSvc, rec.Id, updated, ib1)
	if err == nil {
		t.Fatalf("Update unexpectedly succeeded with partial inbound filter")
	}
	if needRestart {
		t.Fatalf("needRestart = true, want false when update is rejected before mutation")
	}

	kept, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail rollback state: %v", err)
	}
	if kept.Comment != "before" {
		t.Fatalf("client record comment = %q, want %q", kept.Comment, "before")
	}

	c1 := inboundClientByEmail(t, ib1, client.Email)
	c2 := inboundClientByEmail(t, ib2, client.Email)
	if c1.Comment != "before" {
		t.Fatalf("inbound %d comment = %q, want %q", ib1, c1.Comment, "before")
	}
	if c2.Comment != "before" {
		t.Fatalf("inbound %d comment = %q, want %q", ib2, c2.Comment, "before")
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

func TestResetClientTrafficByEmailPrevalidatesAllAttachedInbounds(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "c7049d8b-a70d-4432-9d31-80b2fd93af1a",
		Email:      "reset-direct-prevalidate@example.com",
		SubID:      "sub-reset-direct-prevalidate",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-reset-direct-a", 26543, client)
	ib2 := seedClientMutationInbound(t, "vless-reset-direct-b", 26643, client)

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
		Up:        987,
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

	if err := inboundSvc.ResetClientTrafficByEmail(client.Email); err == nil {
		t.Fatalf("ResetClientTrafficByEmail unexpectedly succeeded")
	}

	var kept xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", client.Email).First(&kept).Error; err != nil {
		t.Fatalf("reload traffic: %v", err)
	}
	if kept.Enable {
		t.Fatalf("traffic enable changed on failed prevalidation")
	}
	if kept.Up != 987 || kept.Down != 654 {
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

func TestGetClientInboundByTrafficIDSkipsStaleTrafficOwnerInbound(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "ab2ee76f-ac28-4288-9d61-8529749b9221",
		Email:      "lookup-traffic@example.com",
		SubID:      "sub-lookup-traffic",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-lookup-traffic-a", 29443, client)
	ib2 := seedClientMutationInbound(t, "vless-lookup-traffic-b", 30443, client)

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
		Up:        33,
		Down:      44,
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

	gotTraffic, gotInbound, err := inboundSvc.GetClientInboundByTrafficID(traffic.Id)
	if err != nil {
		t.Fatalf("GetClientInboundByTrafficID: %v", err)
	}
	if gotTraffic == nil || gotInbound == nil {
		t.Fatalf("GetClientInboundByTrafficID returned nil traffic/inbound")
	}
	if gotTraffic.Email != client.Email {
		t.Fatalf("traffic email = %q, want %q", gotTraffic.Email, client.Email)
	}
	if gotInbound.Id != ib2 {
		t.Fatalf("inbound id = %d, want %d", gotInbound.Id, ib2)
	}
}

func TestGetClientInboundByEmailRejectsLegacyOwnerWithoutMembership(t *testing.T) {
	setupClientMutationDB(t)

	inboundID := seedClientMutationInboundClients(t, "vless-lookup-orphan-email", 31443, nil)
	traffic := &xray.ClientTraffic{
		InboundId: inboundID,
		Email:     "orphan-lookup@example.com",
		Enable:    true,
		Up:        55,
		Down:      66,
	}
	if err := database.GetDB().Create(traffic).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	inboundSvc := &InboundService{}
	gotTraffic, gotInbound, err := inboundSvc.GetClientInboundByEmail(traffic.Email)
	if err == nil {
		t.Fatalf("GetClientInboundByEmail should fail for stale legacy owner without membership")
	}
	if gotTraffic == nil {
		t.Fatalf("GetClientInboundByEmail returned nil traffic")
	}
	if gotInbound != nil {
		t.Fatalf("GetClientInboundByEmail should not return fallback inbound %d for stale membership", gotInbound.Id)
	}
}

func TestGetClientInboundByTrafficIDRejectsLegacyOwnerWithoutMembership(t *testing.T) {
	setupClientMutationDB(t)

	inboundID := seedClientMutationInboundClients(t, "vless-lookup-orphan-traffic", 32443, nil)
	traffic := &xray.ClientTraffic{
		InboundId: inboundID,
		Email:     "orphan-traffic@example.com",
		Enable:    true,
		Up:        77,
		Down:      88,
	}
	if err := database.GetDB().Create(traffic).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	inboundSvc := &InboundService{}
	gotTraffic, gotInbound, err := inboundSvc.GetClientInboundByTrafficID(traffic.Id)
	if err == nil {
		t.Fatalf("GetClientInboundByTrafficID should fail for stale legacy owner without membership")
	}
	if gotTraffic == nil {
		t.Fatalf("GetClientInboundByTrafficID returned nil traffic")
	}
	if gotInbound != nil {
		t.Fatalf("GetClientInboundByTrafficID should not return fallback inbound %d for stale membership", gotInbound.Id)
	}
}

func TestDelInboundPreservesDetachedClientRecordAndTraffic(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "0dd7003a-1984-4f50-89cb-ee48a1af88ae",
		Email:      "delete-inbound-detached@example.com",
		SubID:      "sub-delete-inbound-detached",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-delete-detached", 33453, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
			InboundId:  ib1,
			Email:      client.Email,
			Enable:     true,
			Total:      client.TotalGB,
			ExpiryTime: client.ExpiryTime,
		}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	needRestart, err := inboundSvc.DelInbound(ib1)
	if err != nil {
		t.Fatalf("DelInbound(%d): %v", ib1, err)
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when local runtime is absent")
	}

	if _, err := clientSvc.GetRecordByEmail(nil, client.Email); err != nil {
		t.Fatalf("GetRecordByEmail after DelInbound: %v", err)
	}
	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail second read: %v", err)
	}
	attached, err := clientSvc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(attached) != 0 {
		t.Fatalf("attached inbounds after DelInbound = %v, want detached client", attached)
	}

	traffic, err := inboundSvc.GetClientTrafficByEmail(client.Email)
	if err != nil {
		t.Fatalf("GetClientTrafficByEmail after DelInbound: %v", err)
	}
	if traffic == nil {
		t.Fatalf("GetClientTrafficByEmail returned nil traffic after DelInbound")
	}
	if traffic.UUID != client.ID {
		t.Fatalf("traffic UUID = %q, want %q", traffic.UUID, client.ID)
	}
	if traffic.SubId != client.SubID {
		t.Fatalf("traffic SubId = %q, want %q", traffic.SubId, client.SubID)
	}

	listed, err := clientSvc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List returned %d clients, want 1", len(listed))
	}
	if len(listed[0].InboundIds) != 0 {
		t.Fatalf("List inboundIds = %v, want detached client", listed[0].InboundIds)
	}
	if listed[0].Traffic == nil || listed[0].Traffic.Email != client.Email {
		t.Fatalf("List traffic = %+v, want preserved traffic for %s", listed[0].Traffic, client.Email)
	}
}

func TestDelInboundPreservesRemainingSharedAttachments(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "864e924f-c9be-4434-a298-a16500de7236",
		Email:      "delete-inbound-shared@example.com",
		SubID:      "sub-delete-inbound-shared",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-delete-shared-a", 33463, client)
	ib2 := seedClientMutationInbound(t, "vless-delete-shared-b", 33473, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}

	needRestart, err := inboundSvc.DelInbound(ib1)
	if err != nil {
		t.Fatalf("DelInbound(%d): %v", ib1, err)
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when local runtime is absent")
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail after DelInbound: %v", err)
	}
	attached, err := clientSvc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(attached) != 1 || attached[0] != ib2 {
		t.Fatalf("attached inbounds after DelInbound = %v, want [%d]", attached, ib2)
	}
	if !inboundHasClientEmail(t, ib2, client.Email) {
		t.Fatalf("client missing from surviving inbound %d", ib2)
	}
}

func TestDelDepletedClientsRollsBackInlineInboundDeletion(t *testing.T) {
	setupClientMutationDB(t)

	depletedClient := model.Client{
		ID:         "2f2c0d1f-5f9f-4e8c-83d5-c74dd8aef471",
		Email:      "depleted-inline-delete@example.com",
		SubID:      "sub-depleted-inline-delete",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    100,
		ExpiryTime: 4102444800000,
	}
	blockingClient := model.Client{
		ID:         "f4a8a0b2-4b34-4b5d-8965-7a2382ef2d68",
		Email:      "depleted-inline-block@example.com",
		SubID:      "sub-depleted-inline-block",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    100,
		ExpiryTime: 4102444800000,
	}

	ib1 := seedClientMutationInbound(t, "vless-depleted-inline-a", 33483, depletedClient)
	ib2 := seedClientMutationInbound(t, "vless-depleted-inline-b", 33493, blockingClient)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{depletedClient}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{blockingClient}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId:  ib1,
		Email:      depletedClient.Email,
		Enable:     true,
		Total:      100,
		Up:         100,
		ExpiryTime: depletedClient.ExpiryTime,
	}).Error; err != nil {
		t.Fatalf("seed depleted traffic: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib2).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib2, err)
	}
	corrupt.Settings = "{"
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib2, err)
	}

	if err := inboundSvc.DelDepletedClients(-1); err == nil {
		t.Fatalf("DelDepletedClients unexpectedly succeeded")
	}

	var keptInbound model.Inbound
	if err := database.GetDB().First(&keptInbound, ib1).Error; err != nil {
		t.Fatalf("rollback did not restore inbound %d: %v", ib1, err)
	}
	if !inboundHasClientEmail(t, ib1, depletedClient.Email) {
		t.Fatalf("rollback left inbound %d without client %s", ib1, depletedClient.Email)
	}
	rec, err := clientSvc.GetRecordByEmail(nil, depletedClient.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail after rollback: %v", err)
	}
	attached, err := clientSvc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(attached) != 1 || attached[0] != ib1 {
		t.Fatalf("attached inbounds after rollback = %v, want [%d]", attached, ib1)
	}
}

func TestBulkAdjustFallsBackToAtomicAdjustForMultiAttachClient(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "bf2c98ee-bf15-4f17-8db9-963ad648cc53",
		Email:      "bulk-adjust-shared@example.com",
		SubID:      "sub-bulk-adjust-shared",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Comment:    "before",
	}
	ib1 := seedClientMutationInbound(t, "vless-bulk-adjust-a", 33443, client)
	ib2 := seedClientMutationInbound(t, "vless-bulk-adjust-b", 34443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId:  ib1,
		Email:      client.Email,
		Enable:     true,
		Total:      client.TotalGB,
		ExpiryTime: client.ExpiryTime,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib2).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib2, err)
	}
	corrupt.Settings = `{"clients":`
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib2, err)
	}

	result, _, err := clientSvc.BulkAdjust(inboundSvc, []string{client.Email}, 7, 512)
	if err != nil {
		t.Fatalf("BulkAdjust: %v", err)
	}
	if result.Adjusted != 0 {
		t.Fatalf("BulkAdjust adjusted = %d, want 0 after rollback", result.Adjusted)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("BulkAdjust skipped = %+v, want single rollback failure", result.Skipped)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.TotalGB != client.TotalGB {
		t.Fatalf("client record totalGB = %d, want %d", rec.TotalGB, client.TotalGB)
	}
	if rec.ExpiryTime != client.ExpiryTime {
		t.Fatalf("client record expiry = %d, want %d", rec.ExpiryTime, client.ExpiryTime)
	}

	c1 := inboundClientByEmail(t, ib1, client.Email)
	if c1.TotalGB != client.TotalGB {
		t.Fatalf("inbound %d totalGB = %d, want %d", ib1, c1.TotalGB, client.TotalGB)
	}
	if c1.ExpiryTime != client.ExpiryTime {
		t.Fatalf("inbound %d expiry = %d, want %d", ib1, c1.ExpiryTime, client.ExpiryTime)
	}

	var traffic xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", client.Email).First(&traffic).Error; err != nil {
		t.Fatalf("reload traffic: %v", err)
	}
	if traffic.Total != client.TotalGB {
		t.Fatalf("traffic total = %d, want %d", traffic.Total, client.TotalGB)
	}
	if traffic.ExpiryTime != client.ExpiryTime {
		t.Fatalf("traffic expiry = %d, want %d", traffic.ExpiryTime, client.ExpiryTime)
	}
}

func TestAddToGroupRollsBackWhenInboundSettingsCorrupt(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "879286d8-0f01-464d-b8c5-f8fd35278e54",
		Email:      "group-add-rollback@example.com",
		SubID:      "sub-group-add-rollback",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-group-add-a", 35443, client)

	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib1).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib1, err)
	}
	corrupt.Settings = `{"clients":`
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib1, err)
	}

	if _, err := clientSvc.AddToGroup([]string{client.Email}, "grp-rollback"); err == nil {
		t.Fatalf("AddToGroup unexpectedly succeeded")
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.Group != "" {
		t.Fatalf("client record group = %q, want empty after rollback", rec.Group)
	}

	var count int64
	if err := database.GetDB().Model(&model.ClientGroup{}).Where("name = ?", "grp-rollback").Count(&count).Error; err != nil {
		t.Fatalf("count group: %v", err)
	}
	if count != 0 {
		t.Fatalf("group row count = %d, want 0 after rollback", count)
	}
}

func TestRenameGroupRollsBackWhenInboundSettingsCorrupt(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "8a5598d2-1d2d-4e59-8a4c-f7ce6178727b",
		Email:      "group-rename-rollback@example.com",
		SubID:      "sub-group-rename-rollback",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
		Group:      "oldgrp",
	}
	ib1 := seedClientMutationInbound(t, "vless-group-rename-a", 36443, client)

	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := database.GetDB().Create(&model.ClientGroup{Name: "oldgrp"}).Error; err != nil {
		t.Fatalf("seed group row: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib1).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib1, err)
	}
	corrupt.Settings = `{"clients":`
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib1, err)
	}

	if _, err := clientSvc.RenameGroup("oldgrp", "newgrp"); err == nil {
		t.Fatalf("RenameGroup unexpectedly succeeded")
	}

	rec, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.Group != "oldgrp" {
		t.Fatalf("client record group = %q, want %q after rollback", rec.Group, "oldgrp")
	}

	var oldCount int64
	if err := database.GetDB().Model(&model.ClientGroup{}).Where("name = ?", "oldgrp").Count(&oldCount).Error; err != nil {
		t.Fatalf("count old group: %v", err)
	}
	if oldCount != 1 {
		t.Fatalf("old group count = %d, want 1 after rollback", oldCount)
	}
	var newCount int64
	if err := database.GetDB().Model(&model.ClientGroup{}).Where("name = ?", "newgrp").Count(&newCount).Error; err != nil {
		t.Fatalf("count new group: %v", err)
	}
	if newCount != 0 {
		t.Fatalf("new group count = %d, want 0 after rollback", newCount)
	}
}

func TestBulkDeleteFallsBackToAtomicDeleteForMultiAttachClient(t *testing.T) {
	setupClientMutationDB(t)

	client := model.Client{
		ID:         "eb4b8282-8c20-4c32-b1d8-6c52c11f49eb",
		Email:      "bulk-delete-shared@example.com",
		SubID:      "sub-bulk-delete-shared",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	ib1 := seedClientMutationInbound(t, "vless-bulk-del-a", 31443, client)
	ib2 := seedClientMutationInbound(t, "vless-bulk-del-b", 32443, client)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(database.GetDB(), ib1, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib1: %v", err)
	}
	if err := clientSvc.SyncInbound(database.GetDB(), ib2, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound ib2: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: ib1,
		Email:     client.Email,
		Enable:    true,
		Up:        55,
		Down:      66,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	var corrupt model.Inbound
	if err := database.GetDB().First(&corrupt, ib2).Error; err != nil {
		t.Fatalf("load inbound %d: %v", ib2, err)
	}
	corrupt.Settings = "{"
	if err := database.GetDB().Save(&corrupt).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ib2, err)
	}

	result, _, err := clientSvc.BulkDelete(inboundSvc, []string{client.Email}, false)
	if err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("deleted = %d, want 0 on failed atomic shared delete", result.Deleted)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Email != client.Email {
		t.Fatalf("skipped = %+v, want one skipped email %q", result.Skipped, client.Email)
	}
	if !inboundHasClientEmail(t, ib1, client.Email) {
		t.Fatalf("bulk delete partially removed client from inbound %d", ib1)
	}
	if _, err := clientSvc.GetRecordByEmail(nil, client.Email); err != nil {
		t.Fatalf("client record missing after skipped bulk delete: %v", err)
	}
	if isClientEmailTombstoned(client.Email) {
		t.Fatalf("bulk delete left false tombstone for skipped shared client")
	}
}

func TestBulkCreateNormalizesDuplicateInboundIDs(t *testing.T) {
	setupClientMutationDB(t)

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	ib1 := seedClientMutationInboundClients(t, "vless-bulk-create-norm", 33443, nil)

	result, needRestart, err := clientSvc.BulkCreate(inboundSvc, []ClientCreatePayload{{
		Client: model.Client{
			Email:      "bulk-create-norm@example.com",
			Enable:     true,
			LimitIP:    1,
			TotalGB:    1024,
			ExpiryTime: 4102444800000,
		},
		InboundIds: []int{ib1, ib1, ib1},
	}})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when local inbound mutation happened without active runtime")
	}
	if result.Created != 1 {
		t.Fatalf("created = %d, want 1", result.Created)
	}

	rec, err := clientSvc.GetRecordByEmail(nil, "bulk-create-norm@example.com")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	attached, err := clientSvc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if len(attached) != 1 || attached[0] != ib1 {
		t.Fatalf("attached inbounds = %v, want [%d]", attached, ib1)
	}
	if !inboundHasClientEmail(t, ib1, rec.Email) {
		t.Fatalf("created client missing from inbound %d", ib1)
	}
}
