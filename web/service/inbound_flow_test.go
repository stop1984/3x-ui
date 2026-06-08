package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestSanitizeInboundClientFlowSettings_KeepVlessTCPFlow(t *testing.T) {
	settings := `{
	  "clients": [{"email":"a","id":"u","flow":"xtls-rprx-vision"}],
	  "testseed": [1,2,3,4]
	}`
	stream := `{"network":"tcp","security":"tls"}`

	got, changed, err := sanitizeInboundClientFlowSettings(model.VLESS, stream, settings)
	if err != nil {
		t.Fatalf("sanitize returned error: %v", err)
	}
	if changed {
		t.Fatalf("expected no change for VLESS TCP+TLS vision flow, got changed")
	}
	if got != settings {
		t.Fatalf("unexpected rewrite when nothing changed")
	}
}

func TestSanitizeInboundClientFlowSettings_ClearUnsupportedFlowAndTestseed(t *testing.T) {
	settings := `{
	  "clients": [{"email":"a","id":"u","flow":"xtls-rprx-vision"}],
	  "testseed": [1,2,3,4]
	}`
	stream := `{"network":"ws","security":"tls"}`

	got, changed, err := sanitizeInboundClientFlowSettings(model.VLESS, stream, settings)
	if err != nil {
		t.Fatalf("sanitize returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected unsupported flow to be rewritten")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal rewritten settings: %v", err)
	}
	clients, _ := parsed["clients"].([]any)
	client, _ := clients[0].(map[string]any)
	if flow, _ := client["flow"].(string); flow != "" {
		t.Fatalf("flow = %q, want empty", flow)
	}
	if _, exists := parsed["testseed"]; exists {
		t.Fatal("testseed should be removed when flow is no longer valid")
	}
}

func TestSanitizeInboundClientFlowSettings_ClearTrojanFlow(t *testing.T) {
	settings := `{
	  "clients": [{"email":"a","password":"p","flow":"xtls-rprx-vision"}]
	}`
	stream := `{"network":"tcp","security":"reality"}`

	got, changed, err := sanitizeInboundClientFlowSettings(model.Trojan, stream, settings)
	if err != nil {
		t.Fatalf("sanitize returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected trojan flow to be cleared")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal rewritten settings: %v", err)
	}
	clients, _ := parsed["clients"].([]any)
	client, _ := clients[0].(map[string]any)
	if flow, _ := client["flow"].(string); flow != "" {
		t.Fatalf("flow = %q, want empty", flow)
	}
}

func TestInboundCanEnableTlsFlow(t *testing.T) {
	if !inboundCanEnableTlsFlow(string(model.VLESS), `{"network":"tcp","security":"reality"}`) {
		t.Fatal("vless tcp reality should support tls flow")
	}
	if inboundCanEnableTlsFlow(string(model.VLESS), `{"network":"grpc","security":"reality"}`) {
		t.Fatal("vless grpc reality must not support tls flow")
	}
	if inboundCanEnableTlsFlow(string(model.Trojan), `{"network":"tcp","security":"tls"}`) {
		t.Fatal("trojan must not be marked tls-flow-capable")
	}
}
