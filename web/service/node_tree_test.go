package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"
)

// #4983: a transitive sub-node learned from a direct node must surface as its
// own read-only entry nested under its parent, and per-GUID counts must split a
// direct node's own inbounds from its sub-nodes'.
func TestGetNodeTree_SurfacesTransitiveNodeNestedUnderParent(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	svc := NodeService{}
	selfGuid, _ := (&SettingService{}).GetPanelGuid()

	if err := db.Create(&model.Node{
		Id: 1, Name: "Node2", Address: "10.0.0.2", Port: 2053,
		ApiToken: "t", Guid: "node2-guid", Status: "online",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	// Node2's own inbound and a transitive inbound physically on Node3
	// (managed through Node2, so node_id = Node2 but origin = Node3).
	nid := 1
	if err := db.Create(&model.Inbound{Tag: "n1-own", Enable: true, Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`, NodeID: &nid, OriginNodeGuid: "node2-guid"}).Error; err != nil {
		t.Fatalf("create own inbound: %v", err)
	}
	if err := db.Create(&model.Inbound{Tag: "n1-via", Enable: true, Port: 8443, Protocol: model.VLESS, Settings: `{"clients":[]}`, NodeID: &nid, OriginNodeGuid: "node3-guid"}).Error; err != nil {
		t.Fatalf("create transitive inbound: %v", err)
	}

	// The heartbeat learned that Node2 manages Node3.
	nodeDescendantsMu.Lock()
	nodeDescendantsCache[1] = []model.NodeSummary{{
		Guid: "node3-guid", ParentGuid: "node2-guid", Name: "Node3", Address: "10.0.0.3", Status: "online",
	}}
	nodeDescendantsMu.Unlock()
	t.Cleanup(func() {
		nodeDescendantsMu.Lock()
		nodeDescendantsCache = map[int][]model.NodeSummary{}
		nodeDescendantsMu.Unlock()
	})

	tree, err := svc.GetNodeTree()
	if err != nil {
		t.Fatalf("GetNodeTree: %v", err)
	}

	var node2, node3 *model.Node
	for _, n := range tree {
		switch n.Guid {
		case "node2-guid":
			node2 = n
		case "node3-guid":
			node3 = n
		}
	}
	if node2 == nil || node3 == nil {
		t.Fatalf("expected Node2 + transitive Node3, got %d nodes", len(tree))
	}
	if node2.ParentGuid != selfGuid {
		t.Errorf("Node2 parent = %q, want this panel's GUID %q", node2.ParentGuid, selfGuid)
	}
	if !node3.Transitive || node3.ParentGuid != "node2-guid" {
		t.Errorf("Node3 should be transitive under node2-guid, got transitive=%v parent=%q", node3.Transitive, node3.ParentGuid)
	}
	if node3.Id != 0 {
		t.Errorf("transitive node must be a read-only projection (Id 0), got Id=%d", node3.Id)
	}
	if node2.InboundCount != 1 {
		t.Errorf("Node2 should host only its own inbound, got InboundCount=%d", node2.InboundCount)
	}
	if node3.InboundCount != 1 {
		t.Errorf("transitive Node3 should host its 1 inbound, got %d", node3.InboundCount)
	}
}

// Shared client_traffics rows are email-keyed and can point at a sibling local
// inbound even when the client is also attached to a node-owned inbound. Node
// depleted counts must follow node membership, not the stale owner inbound id.
func TestGetAllCountsDepletedNodeClientsByMembership(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	if err := db.Create(&model.Node{
		Id: 1, Name: "Node1", Address: "10.0.0.2", Port: 2053,
		ApiToken: "t", Guid: "node1-guid", Status: "online",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	localInbound := &model.Inbound{
		UserId:         1,
		Tag:            "local-shared",
		Enable:         true,
		Port:           48001,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{"id":"64d6427b-7c01-462f-8a8a-a525778cc8f8","email":"node-shared@example.com","enable":true}]}`,
	}
	nodeID := 1
	nodeInbound := &model.Inbound{
		UserId:         1,
		Tag:            "node-shared",
		Enable:         true,
		Port:           48002,
		Protocol:       model.VLESS,
		NodeID:         &nodeID,
		OriginNodeGuid: "node1-guid",
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{"id":"64d6427b-7c01-462f-8a8a-a525778cc8f8","email":"node-shared@example.com","enable":true}]}`,
	}
	if err := db.Create(localInbound).Error; err != nil {
		t.Fatalf("create local inbound: %v", err)
	}
	if err := db.Create(nodeInbound).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}

	client := model.Client{
		ID:         "64d6427b-7c01-462f-8a8a-a525778cc8f8",
		Email:      "node-shared@example.com",
		SubID:      "sub-node-shared",
		Enable:     true,
		LimitIP:    1,
		TotalGB:    1024,
		ExpiryTime: 4102444800000,
	}
	inboundSvc := InboundService{}
	if err := inboundSvc.clientService.SyncInbound(db, localInbound.Id, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound local: %v", err)
	}
	if err := inboundSvc.clientService.SyncInbound(db, nodeInbound.Id, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound node: %v", err)
	}

	if err := db.Create(&xray.ClientTraffic{
		InboundId:  localInbound.Id,
		Email:      client.Email,
		Enable:     false,
		ExpiryTime: 1,
	}).Error; err != nil {
		t.Fatalf("create shared traffic: %v", err)
	}

	nodes, err := (&NodeService{}).GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}

	var node *model.Node
	for _, n := range nodes {
		if n.Id == 1 {
			node = n
			break
		}
	}
	if node == nil {
		t.Fatalf("node 1 not returned")
	}
	if node.ClientCount != 1 {
		t.Fatalf("node client count = %d, want 1", node.ClientCount)
	}
	if node.DepletedCount != 1 {
		t.Fatalf("node depleted count = %d, want 1", node.DepletedCount)
	}
}
