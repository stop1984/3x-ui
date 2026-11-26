package sub

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	xraynet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/routing"
	routingsession "github.com/xtls/xray-core/features/routing/session"
	"github.com/xtls/xray-core/infra/conf"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
)

// Exercise the same exported profiles that a client reads, with a real Xray
// server and client and a TLS origin all on loopback.
func TestXHTTPRealitySubscriptionConnects(t *testing.T) {
	const response = "through VLESS XHTTP"
	const subID = "xhttp-reality-e2e"
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, response)
	}))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	t.Cleanup(origin.Close)
	originPort := origin.Listener.Addr().(*net.TCPAddr).Port
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	stream := map[string]any{
		"network": "xhttp", "security": "reality",
		"xhttpSettings": map[string]any{"path": "/subscription-test", "mode": "auto", "host": ""},
		"realitySettings": map[string]any{
			"target":      origin.Listener.Addr().String(),
			"serverNames": []string{"example.com"},
			"privateKey":  base64.RawURLEncoding.EncodeToString(key.Bytes()),
			"shortIds":    []string{"a1b2c3d4"},
			"settings": map[string]any{
				"publicKey":   base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
				"fingerprint": "chrome",
			},
		},
	}
	streamJSON, err := json.Marshal(stream)
	if err != nil {
		t.Fatal(err)
	}
	seedSubDB(t)
	serverPort := freeXHTTPPort(t)
	ib := seedSubInbound(t, subID, "xhttp-e2e", serverPort, 1, string(streamJSON))
	if err := database.GetDB().Model(ib).Update("listen", "127.0.0.1").Error; err != nil {
		t.Fatal(err)
	}
	startXHTTPCore(t, map[string]any{
		"listen": "127.0.0.1", "port": serverPort, "protocol": "vless",
		"settings": json.RawMessage(ib.Settings), "streamSettings": stream,
	}, map[string]any{
		"protocol": "freedom",
		// Xray blocks loopback destinations by default. Allow only this fixture.
		"settings": map[string]any{"finalRules": []map[string]any{{
			"action": "allow", "ip": []string{"127.0.0.1/32"}, "port": originPort,
		}}},
	})

	panel := httptest.NewServer(newSubscriptionTestRouter(subscriptionTestRouterConfig{jsonAlwaysArray: true}))
	t.Cleanup(panel.Close)

	t.Run("raw", func(t *testing.T) {
		body := readXHTTPBody(t, panel.Client(), panel.URL+"/sub/"+subID)
		outbounds, _, err := link.ParseSubscriptionBody(body)
		if err != nil || len(outbounds) != 1 {
			t.Fatalf("raw subscription: got %d outbounds, err=%v", len(outbounds), err)
		}
		assertXHTTPSubscriptionRequest(t, origin, outbounds[0], response)
	})
	t.Run("json", func(t *testing.T) {
		body := readXHTTPBody(t, panel.Client(), panel.URL+"/json/"+subID)
		var profiles []json.RawMessage
		if err := json.Unmarshal(body, &profiles); err != nil {
			t.Fatalf("JSON subscription: %v", err)
		}
		if len(profiles) != 1 {
			t.Fatalf("JSON subscription: got %d profiles, want one", len(profiles))
		}
		var profile struct {
			Outbounds []map[string]any `json:"outbounds"`
		}
		if err := json.Unmarshal(profiles[0], &profile); err != nil {
			t.Fatalf("JSON subscription profile: %v", err)
		}
		for _, outbound := range profile.Outbounds {
			if outbound["protocol"] == "vless" {
				tag, _ := outbound["tag"].(string)
				if tag == "" {
					t.Fatal("JSON subscription VLESS outbound has no tag")
				}
				t.Run("profile", func(t *testing.T) {
					assertXHTTPJSONProfile(t, profiles[0], tag, originPort)
				})
				t.Run("request", func(t *testing.T) {
					assertXHTTPSubscriptionRequest(t, origin, outbound, response)
				})
				return
			}
		}
		t.Fatal("JSON subscription has no VLESS outbound")
	})
}

func assertXHTTPJSONProfile(t *testing.T, profile json.RawMessage, proxyTag string, originPort int) {
	t.Helper()
	// Initialize the profile unchanged, but don't start its fixed-port listeners.
	x := newXHTTPCore(t, profile)
	router := x.GetFeature(routing.RouterType()).(routing.Router)
	route, err := router.PickRoute(&routingsession.Context{
		Inbound:  &session.Inbound{Tag: "mixed"},
		Outbound: &session.Outbound{Target: xraynet.TCPDestination(xraynet.LocalHostIP, xraynet.Port(originPort))},
	})
	if err != nil {
		t.Fatalf("route HTTPS through full JSON subscription profile: %v", err)
	}
	if tag := route.GetOutboundTag(); tag != proxyTag {
		t.Fatalf("JSON subscription routes HTTPS to %q, want VLESS outbound %q", tag, proxyTag)
	}
}

func readXHTTPBody(t *testing.T, client *http.Client, address string) []byte {
	t.Helper()
	resp, err := client.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %s", address, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func assertXHTTPSubscriptionRequest(t *testing.T, origin *httptest.Server, outbound any, want string) {
	t.Helper()
	socksPort := freeXHTTPPort(t)
	// Use only the exported VLESS outbound so a broken export cannot fall back
	// to the JSON profile's direct outbound and still reach the test origin.
	startXHTTPCore(t, map[string]any{
		"listen": "127.0.0.1", "port": socksPort, "protocol": "socks",
	}, outbound)
	transport := origin.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(&url.URL{Scheme: "socks5", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(socksPort))})
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	if body := readXHTTPBody(t, client, origin.URL); string(body) != want {
		t.Fatalf("proxied body = %q, want %q", body, want)
	}
}

func freeXHTTPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func startXHTTPCore(t *testing.T, inbound, outbound any) {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"log":       map[string]string{"loglevel": "none"},
		"inbounds":  []any{inbound},
		"outbounds": []any{outbound},
	})
	if err != nil {
		t.Fatal(err)
	}
	x := newXHTTPCore(t, data)
	if err := x.Start(); err != nil {
		t.Fatalf("start Xray instance: %v", err)
	}
}

func newXHTTPCore(t *testing.T, data []byte) *core.Instance {
	t.Helper()
	var config conf.Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	built, err := config.Build()
	if err != nil {
		t.Fatalf("build Xray config: %v", err)
	}
	x, err := core.New(built)
	if err != nil {
		t.Fatalf("create Xray instance: %v", err)
	}
	t.Cleanup(func() { _ = x.Close() })
	return x
}
