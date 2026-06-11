package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"
)

func mkTraffic(t *testing.T, inboundId int, email string, up, down, total, expiry int64, enable bool) {
	t.Helper()
	row := xray.ClientTraffic{
		InboundId:  inboundId,
		Email:      email,
		Up:         up,
		Down:       down,
		Total:      total,
		ExpiryTime: expiry,
		Enable:     enable,
	}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatalf("create traffic %s: %v", email, err)
	}
}

func trafficOf(t *testing.T, email string) xray.ClientTraffic {
	t.Helper()
	var row xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("load traffic %s: %v", email, err)
	}
	return row
}

func TestBulkResetTrafficZeroesUsageAndReenables(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
		{Email: "bob@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "sb", Enable: true},
		{Email: "carol@x", ID: "33333333-3333-3333-3333-333333333333", SubID: "sc", Enable: true},
	}
	ib := mkInbound(t, 21001, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, "alice@x", 10, 20, 0, 0, false)
	mkTraffic(t, ib.Id, "bob@x", 5, 5, 0, 0, true)
	mkTraffic(t, ib.Id, "carol@x", 7, 0, 0, 0, true)

	affected, needRestart, err := svc.BulkResetTraffic(inboundSvc, []string{"alice@x", "bob@x"})
	if err != nil {
		t.Fatalf("BulkResetTraffic: %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected 2 affected, got %d", affected)
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when runtime-add fallback is needed for a traffic-only disabled row")
	}

	for _, e := range []string{"alice@x", "bob@x"} {
		tr := trafficOf(t, e)
		if tr.Up != 0 || tr.Down != 0 {
			t.Fatalf("%s: expected up/down 0, got up=%d down=%d", e, tr.Up, tr.Down)
		}
		if !tr.Enable {
			t.Fatalf("%s: expected re-enabled", e)
		}
	}

	carol := trafficOf(t, "carol@x")
	if carol.Up != 7 {
		t.Fatalf("carol not in list should be untouched, got up=%d", carol.Up)
	}
}

func TestBulkResetTrafficReenablesCanonicalClientState(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
	}
	ib := mkInbound(t, 21011, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, "alice@x", 10, 20, 0, 0, false)

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("email = ?", "alice@x").
		Updates(map[string]any{"enable": false}).Error; err != nil {
		t.Fatalf("disable client record: %v", err)
	}
	if _, _, err := inboundSvc.markClientsDisabledInSettings(database.GetDB(), ib.Id, map[string]struct{}{"alice@x": {}}); err != nil {
		t.Fatalf("markClientsDisabledInSettings: %v", err)
	}

	affected, needRestart, err := svc.BulkResetTraffic(inboundSvc, []string{"alice@x"})
	if err != nil {
		t.Fatalf("BulkResetTraffic: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 affected, got %d", affected)
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when canonical re-enable hits local runtime-absent path")
	}

	tr := trafficOf(t, "alice@x")
	if tr.Up != 0 || tr.Down != 0 {
		t.Fatalf("alice: expected up/down 0, got up=%d down=%d", tr.Up, tr.Down)
	}
	if !tr.Enable {
		t.Fatalf("alice: expected traffic re-enabled")
	}

	rec, err := svc.GetRecordByEmail(nil, "alice@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if !rec.Enable {
		t.Fatalf("alice client record remained disabled")
	}

	reloaded, err := inboundSvc.GetInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	jsonClients, err := inboundSvc.GetClients(reloaded)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if len(jsonClients) != 1 || !jsonClients[0].Enable {
		t.Fatalf("embedded client remained disabled after bulk reset")
	}
}

func TestResetAllTrafficsReenablesCanonicalClientState(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
		{Email: "bob@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "sb", Enable: true},
	}
	ib := mkInbound(t, 21012, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, "alice@x", 10, 20, 0, 0, false)
	mkTraffic(t, ib.Id, "bob@x", 7, 8, 0, 0, true)

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("email = ?", "alice@x").
		Updates(map[string]any{"enable": false}).Error; err != nil {
		t.Fatalf("disable client record: %v", err)
	}
	if _, _, err := inboundSvc.markClientsDisabledInSettings(database.GetDB(), ib.Id, map[string]struct{}{"alice@x": {}}); err != nil {
		t.Fatalf("markClientsDisabledInSettings: %v", err)
	}

	needRestart, err := svc.ResetAllTraffics(inboundSvc)
	if err != nil {
		t.Fatalf("ResetAllTraffics: %v", err)
	}
	if !needRestart {
		t.Fatalf("needRestart = false, want true when global reset re-enables a disabled local client without runtime")
	}

	for _, email := range []string{"alice@x", "bob@x"} {
		tr := trafficOf(t, email)
		if tr.Up != 0 || tr.Down != 0 {
			t.Fatalf("%s: expected up/down 0, got up=%d down=%d", email, tr.Up, tr.Down)
		}
		if !tr.Enable {
			t.Fatalf("%s: expected traffic re-enabled", email)
		}
	}

	rec, err := svc.GetRecordByEmail(nil, "alice@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if !rec.Enable {
		t.Fatalf("alice client record remained disabled")
	}

	reloaded, err := inboundSvc.GetInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	jsonClients, err := inboundSvc.GetClients(reloaded)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if len(jsonClients) != 2 {
		t.Fatalf("expected 2 embedded clients, got %d", len(jsonClients))
	}
	for _, c := range jsonClients {
		if c.Email == "alice@x" && !c.Enable {
			t.Fatalf("alice remained disabled in inbound settings after global reset")
		}
	}
}

func TestGetClientTrafficTgBotUsesCanonicalClientRecords(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
	}
	ib := mkInbound(t, 21013, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, "alice@x", 1, 2, 0, 0, true)

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("email = ?", "alice@x").
		Update("tg_id", int64(424242)).Error; err != nil {
		t.Fatalf("set tg_id on client record: %v", err)
	}

	traffics, err := inboundSvc.GetClientTrafficTgBot(424242)
	if err != nil {
		t.Fatalf("GetClientTrafficTgBot: %v", err)
	}
	if len(traffics) != 1 {
		t.Fatalf("expected 1 traffic, got %d", len(traffics))
	}
	if traffics[0].Email != "alice@x" {
		t.Fatalf("unexpected email %q", traffics[0].Email)
	}
	if traffics[0].UUID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("UUID not enriched from client record, got %q", traffics[0].UUID)
	}
	if traffics[0].SubId != "sa" {
		t.Fatalf("SubId not enriched from client record, got %q", traffics[0].SubId)
	}
}

func TestDelDepletedRemovesOnlyDepleted(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
		{Email: "bob@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "sb", Enable: true},
		{Email: "carol@x", ID: "33333333-3333-3333-3333-333333333333", SubID: "sc", Enable: true},
	}
	ib := mkInbound(t, 21002, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	past := time.Now().Add(-time.Hour).UnixMilli()
	mkTraffic(t, ib.Id, "alice@x", 60, 60, 100, 0, true)
	mkTraffic(t, ib.Id, "bob@x", 10, 10, 100, 0, true)
	mkTraffic(t, ib.Id, "carol@x", 0, 0, 0, past, true)

	deleted, _, err := svc.DelDepleted(inboundSvc)
	if err != nil {
		t.Fatalf("DelDepleted: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted (alice traffic-depleted, carol expired), got %d", deleted)
	}

	if _, err := svc.GetRecordByEmail(nil, "bob@x"); err != nil {
		t.Fatalf("bob should survive: %v", err)
	}
	for _, e := range []string{"alice@x", "carol@x"} {
		if _, err := svc.GetRecordByEmail(nil, e); err == nil {
			t.Fatalf("%s should be deleted", e)
		}
	}

	reloaded, _ := inboundSvc.GetInbound(ib.Id)
	jsonClients, _ := inboundSvc.GetClients(reloaded)
	if len(jsonClients) != 1 || jsonClients[0].Email != "bob@x" {
		t.Fatalf("settings JSON should contain only bob, got %d clients", len(jsonClients))
	}
}

func TestGetClientTrafficByEmailReadsClientsTable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
	}
	ib := mkInbound(t, 21003, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, "alice@x", 1, 2, 0, 0, true)

	tr, err := inboundSvc.GetClientTrafficByEmail("alice@x")
	if err != nil {
		t.Fatalf("GetClientTrafficByEmail: %v", err)
	}
	if tr == nil {
		t.Fatalf("expected traffic, got nil")
	}
	if tr.UUID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("UUID not enriched from clients table, got %q", tr.UUID)
	}
	if tr.SubId != "sa" {
		t.Fatalf("SubId not enriched from clients table, got %q", tr.SubId)
	}
}
