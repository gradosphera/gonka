package inference

import (
	"context"
	"sort"
	"testing"

	"github.com/productscience/inference/x/inference/keeper"

	"github.com/productscience/inference/x/inference/types"
	"github.com/stretchr/testify/require"
)

// Mock Keeper
type mockKeeperForModelAssigner struct {
	hardwareNodes    map[string]*types.HardwareNodes
	governanceModels []types.Model
}

func (m *mockKeeperForModelAssigner) GetGovernanceModelsSorted(ctx context.Context) ([]*types.Model, error) {
	return keeper.ValuesToPointers(m.governanceModels), nil
}

func (m *mockKeeperForModelAssigner) GetHardwareNodes(ctx context.Context, participantId string) (*types.HardwareNodes, bool) {
	nodes, found := m.hardwareNodes[participantId]
	return nodes, found
}

func (m *mockKeeperForModelAssigner) GetActiveParticipants(ctx context.Context, epochId uint64) (val types.ActiveParticipants, found bool) {
	// Not implemented for this mock
	return types.ActiveParticipants{}, false
}

// Mock Logger
type mockLogger struct{}

func (m mockLogger) LogInfo(msg string, subSystem types.SubSystem, keyvals ...interface{})  {}
func (m mockLogger) LogError(msg string, subSystem types.SubSystem, keyvals ...interface{}) {}
func (m mockLogger) LogWarn(msg string, subSystem types.SubSystem, keyvals ...interface{})  {}
func (m mockLogger) LogDebug(msg string, subSystem types.SubSystem, keyvals ...interface{}) {}

func TestSetModelsForParticipants_OneModelTwoNodes_Bug(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	participantAddress := "gonka1xmwh48ugfvd2ktmy0t90ueuzqxdk4g0anwe3v6"
	modelID := "Qwen/QwQ-32B"

	models := []types.Model{
		{
			ProposedBy:             "genesis",
			Id:                     "Qwen/QwQ-32B",
			UnitsOfComputePerToken: 1000,
			HfRepo:                 "Qwen/QwQ-32B",
			HfCommit:               "976055f8c83f394f35dbd3ab09a285a984907bd0",
			ModelArgs:              []string{"--quantization", "fp8", "-kv-cache-dtype", "fp8"},
			VRam:                   32,
			ThroughputPerNonce:     1000,
			ValidationThreshold:    &types.Decimal{Value: 85, Exponent: -2},
		},
		{
			ProposedBy:             "genesis",
			Id:                     "Qwen/Qwen2.5-7B-Instruct",
			UnitsOfComputePerToken: 100,
			HfRepo:                 "Qwen/Qwen2.5-7B-Instruct",
			HfCommit:               "a09a35458c702b33eeacc393d103063234e8bc28",
			ModelArgs:              []string{"--quantization", "fp8"},
			VRam:                   16,
			ThroughputPerNonce:     10000,
			ValidationThreshold:    &types.Decimal{Value: 85, Exponent: -2},
		},
	}
	// Mock Keeper setup
	mockKeeper := &mockKeeperForModelAssigner{
		governanceModels: models,
		hardwareNodes: map[string]*types.HardwareNodes{
			participantAddress: {
				Participant: participantAddress,
				HardwareNodes: []*types.HardwareNode{
					{LocalId: "mlnode1", Models: []string{modelID}},
					{LocalId: "mlnode2", Models: []string{modelID}},
				},
			},
		},
	}

	// Model Assigner
	modelAssigner := NewModelAssigner(mockKeeper, mockLogger{})

	// Participant data setup
	participants := []*types.ActiveParticipant{
		{
			Index:  participantAddress,
			Models: []string{modelID},
			MlNodes: []*types.ModelMLNodes{ // This is the initial state before model assignment
				{
					MlNodes: []*types.MLNodeInfo{
						{NodeId: "mlnode1", PocWeight: 29},
						{NodeId: "mlnode2", PocWeight: 28},
					},
				},
			},
		},
	}

	upcomingEpoch := types.Epoch{Index: 1}

	// 2. Execute
	modelAssigner.setModelsForParticipants(ctx, participants, upcomingEpoch)

	// 3. Assert
	participant := participants[0]

	// The bug causes the model list to have 1 model, but the ml_nodes list has 2 entries.
	// One for the assigned model, and one for the "overflow" node.
	require.Len(t, participant.Models, 1, "Should have one supported model")
	require.Equal(t, modelID, participant.Models[0], "The supported model should be correct")

	require.Len(t, participant.MlNodes, 1, "Should have one MLNode groups corresponding to the model: "+modelID)

	// Check first group (assigned model)
	modelGroup := participant.MlNodes[0]
	require.Len(t, modelGroup.MlNodes, 2, "The model-specific group should have two nodes")

	// Verify that both nodes are in the same group and have the correct timeslot allocations.
	assertNodeInGroup(t, modelGroup.MlNodes, "mlnode1")
	assertNodeInGroup(t, modelGroup.MlNodes, "mlnode2")

	// Verify that one node is allocated for PoC and the other is not.
	assertTimeslotAllocationCount(t, modelGroup.MlNodes, []bool{true, false}, 1)
	assertTimeslotAllocationCount(t, modelGroup.MlNodes, []bool{true, true}, 1)
}

// assertNodeInGroup checks if a node with the given ID exists in the list of nodes.
func assertNodeInGroup(t *testing.T, nodes []*types.MLNodeInfo, nodeID string) {
	t.Helper()
	found := false
	for _, node := range nodes {
		if node.NodeId == nodeID {
			found = true
			break
		}
	}
	require.True(t, found, "Node with ID %s not found in the group", nodeID)
}

// assertTimeslotAllocationCount checks if there are exactly `expectedCount` nodes
// with the given timeslot allocation.
func assertTimeslotAllocationCount(t *testing.T, nodes []*types.MLNodeInfo, allocation []bool, expectedCount int) {
	t.Helper()
	count := 0
	for _, node := range nodes {
		if equalBoolSlice(node.TimeslotAllocation, allocation) {
			count++
		}
	}
	require.Equal(t, expectedCount, count, "Expected %d nodes with timeslot allocation %v, but found %d", expectedCount, allocation, count)
}

// equalBoolSlice compares two boolean slices for equality.
func equalBoolSlice(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSetModelsForParticipants_OneNodeOneModel(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	participantAddress := "gonka1xmwh48ugfvd2ktmy0t90ueuzqxdk4g0anwe3v6"
	modelID := "Qwen/Qwen2.5-7B-Instruct"

	models := []types.Model{
		{
			ProposedBy: "genesis",
			Id:         modelID,
			VRam:       16,
		},
	}
	// Mock Keeper setup
	mockKeeper := &mockKeeperForModelAssigner{
		governanceModels: models,
		hardwareNodes: map[string]*types.HardwareNodes{
			participantAddress: {
				Participant: participantAddress,
				HardwareNodes: []*types.HardwareNode{
					{LocalId: "mlnode1", Models: []string{modelID}},
				},
			},
		},
	}

	// Model Assigner
	modelAssigner := NewModelAssigner(mockKeeper, mockLogger{})

	// Participant data setup
	participants := []*types.ActiveParticipant{
		{
			Index:  participantAddress,
			Models: []string{modelID},
			MlNodes: []*types.ModelMLNodes{
				{
					MlNodes: []*types.MLNodeInfo{
						{NodeId: "mlnode1", PocWeight: 29},
					},
				},
			},
		},
	}

	upcomingEpoch := types.Epoch{Index: 1}

	// 2. Execute
	modelAssigner.setModelsForParticipants(ctx, participants, upcomingEpoch)

	// 3. Assert
	participant := participants[0]

	require.Len(t, participant.Models, 1, "Should have one supported model")
	require.Equal(t, modelID, participant.Models[0], "The supported model should be correct")

	require.Len(t, participant.MlNodes, 1, "Should have one MLNode group corresponding to the model")

	modelGroup := participant.MlNodes[0]
	require.Len(t, modelGroup.MlNodes, 1, "The model-specific group should have one node")

	assertNodeInGroup(t, modelGroup.MlNodes, "mlnode1")
	assertTimeslotAllocationCount(t, modelGroup.MlNodes, []bool{true, false}, 1)
}

func TestSetModelsForParticipants_ManyNodesManyModels(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	participantAddress := "gonka1xmwh48ugfvd2ktmy0t90ueuzqxdk4g0anwe3v6"
	modelA := "Qwen/QwQ-32B"
	modelB := "Qwen/Qwen2.5-7B-Instruct"

	models := []types.Model{
		{ProposedBy: "genesis", Id: modelA, VRam: 32},
		{ProposedBy: "genesis", Id: modelB, VRam: 16},
	}

	// Mock Keeper setup with 4 nodes supporting mixed models
	mockKeeper := &mockKeeperForModelAssigner{
		governanceModels: models,
		hardwareNodes: map[string]*types.HardwareNodes{
			participantAddress: {
				Participant: participantAddress,
				HardwareNodes: []*types.HardwareNode{
					{LocalId: "mlnode1", Models: []string{modelA, modelB}}, // supports both
					{LocalId: "mlnode2", Models: []string{modelA}},         // supports A
					{LocalId: "mlnode3", Models: []string{modelB}},         // supports B
					{LocalId: "mlnode4", Models: []string{modelA, modelB}}, // supports both
				},
			},
		},
	}

	// Model Assigner
	modelAssigner := NewModelAssigner(mockKeeper, mockLogger{})

	// Participant data setup with legacy MLNodes list (pre-assignment state)
	participants := []*types.ActiveParticipant{
		{
			Index:  participantAddress,
			Models: []string{modelA, modelB},
			MlNodes: []*types.ModelMLNodes{
				{
					MlNodes: []*types.MLNodeInfo{
						{NodeId: "mlnode1", PocWeight: 30},
						{NodeId: "mlnode2", PocWeight: 25},
						{NodeId: "mlnode3", PocWeight: 20},
						{NodeId: "mlnode4", PocWeight: 25},
					},
				},
			},
		},
	}

	upcomingEpoch := types.Epoch{Index: 2}

	// 2. Execute
	modelAssigner.setModelsForParticipants(ctx, participants, upcomingEpoch)

	// 3. Assert
	participant := participants[0]

	// Expect two supported models in the same order as governance models
	require.Len(t, participant.Models, 2, "Should have two supported models")
	require.Equal(t, modelA, participant.Models[0], "First model should be modelA")
	require.Equal(t, modelB, participant.Models[1], "Second model should be modelB")

	// Expect two MLNode groups, one per model (no overflow group expected because all nodes get assigned)
	require.Len(t, participant.MlNodes, 2, "Should have two MLNode groups corresponding to the two models")

	// Group for modelA should contain nodes that support A and were unassigned at that time
	groupA := participant.MlNodes[0]
	require.Len(t, groupA.MlNodes, 3, "Model A group should have three nodes (mlnode1, mlnode2, mlnode4)")
	assertNodeInGroup(t, groupA.MlNodes, "mlnode1")
	assertNodeInGroup(t, groupA.MlNodes, "mlnode2")
	assertNodeInGroup(t, groupA.MlNodes, "mlnode4")

	// Group for modelB should contain the remaining node supporting B only
	groupB := participant.MlNodes[1]
	require.Len(t, groupB.MlNodes, 1, "Model B group should have one node (mlnode3)")
	assertNodeInGroup(t, groupB.MlNodes, "mlnode3")

	// Check 50% allocation per group: floor(3/2)=1 inference for A; floor(1/2)=0 for B
	assertTimeslotAllocationCount(t, groupA.MlNodes, []bool{true, true}, 1)
	assertTimeslotAllocationCount(t, groupA.MlNodes, []bool{true, false}, 2)
	assertTimeslotAllocationCount(t, groupB.MlNodes, []bool{true, true}, 0)
	assertTimeslotAllocationCount(t, groupB.MlNodes, []bool{true, false}, 1)
}

// noopKeeper satisfies KeeperForModelAssigner but is not used by distributeLegacyWeight directly.
type noopKeeper struct{}

func (noopKeeper) GetGovernanceModelsSorted(ctx context.Context) ([]*types.Model,
	error) {
	return nil, nil
}
func (noopKeeper) GetHardwareNodes(ctx context.Context, participantId string) (*types.HardwareNodes, bool) {
	return nil, false
}
func (noopKeeper) GetActiveParticipants(ctx context.Context, epochId uint64) (val types.ActiveParticipants, found bool) {
	return types.ActiveParticipants{}, false
}

// This test demonstrates the current buggy behavior: it double-applies the remainder
// when there are no preserved nodes, resulting in an over-allocation.
func TestDistributeLegacyWeight_RemainderBug_NoPreserved(t *testing.T) {
	assigner := NewModelAssigner(noopKeeper{}, mockLogger{})
	// Legacy placeholder with total weight 10
	original := []*types.MLNodeInfo{{NodeId: "", PocWeight: 10}}
	// 4 hardware nodes; current buggy code yields weights {4,4,2,2} (sum 12)
	hw := &types.HardwareNodes{HardwareNodes: []*types.HardwareNode{{LocalId: "n1"},
		{LocalId: "n2"}, {LocalId: "n3"}, {LocalId: "n4"}}}
	out := assigner.distributeLegacyWeight(original, hw,
		map[string]*types.MLNodeInfo{})
	if len(out) != 4 {
		t.Fatalf("expected 4 ML nodes, got %d", len(out))
	}
	var weights []int
	var sum int
	for _, n := range out {
		weights = append(weights, int(n.PocWeight))
		sum += int(n.PocWeight)
	}
	sort.Ints(weights)
	require.Equal(t, 10, sum)
	require.EqualValues(t, []int{2, 2, 3, 3}, weights)
}

// This test demonstrates the current buggy behavior in the presence of a preserved node.
// The pre-increment is lost on the preserved node (without advancing the counter), and the first
// non-preserved node receives +2, resulting in non-preserved weights {5,3} for total = 7 across 2 targets.
func TestDistributeLegacyWeight_RemainderBug_WithPreserved(t *testing.T) {
	assigner := NewModelAssigner(noopKeeper{}, mockLogger{})
	// Legacy placeholder with total weight 7
	original := []*types.MLNodeInfo{{NodeId: "", PocWeight: 7}}
	// Hardware nodes in order: preserved first to exercise the bug deterministically
	hw := &types.HardwareNodes{HardwareNodes: []*types.HardwareNode{{LocalId: "n1"},
		{LocalId: "n2"}, {LocalId: "n3"}}}
	preserved := map[string]*types.MLNodeInfo{
		"n1": {NodeId: "n1", PocWeight: 100, TimeslotAllocation: []bool{true,
			true}},
	}
	out := assigner.distributeLegacyWeight(original, hw, preserved)
	if len(out) != 3 {
		t.Fatalf("expected 3 ML nodes, got %d", len(out))
	}
	var nonPreserved []int
	for _, n := range out {
		if n.NodeId == "n1" {
			if n.PocWeight != 100 {
				t.Fatalf("preserved node weight should remain 100, got %d",
					n.PocWeight)
			}
			continue
		}
		nonPreserved = append(nonPreserved, int(n.PocWeight))
	}
	sort.Ints(nonPreserved)
	require.EqualValues(t, []int{3, 4}, nonPreserved)
}

func TestDistributeLegacyWeight_LowWeight(t *testing.T) {
	assigner := NewModelAssigner(noopKeeper{}, mockLogger{})
	// Legacy placeholder with total weight 2 (less than number of nodes)
	original := []*types.MLNodeInfo{{NodeId: "", PocWeight: 2}}
	// 5 hardware nodes
	hw := &types.HardwareNodes{HardwareNodes: []*types.HardwareNode{
		{LocalId: "n1"},
		{LocalId: "n2"},
		{LocalId: "n3"},
		{LocalId: "n4"},
		{LocalId: "n5"},
	}}
	out := assigner.distributeLegacyWeight(original, hw, map[string]*types.MLNodeInfo{})

	require.Len(t, out, 2, "should have 2 nodes (power remainder)")

	var weights []int
	var sum int
	for _, n := range out {
		weights = append(weights, int(n.PocWeight))
		sum += int(n.PocWeight)
	}
	sort.Ints(weights)

	require.Equal(t, 2, sum, "total weight should remain 2")
	require.NotEqual(t, []int{0, 0, 0, 0, 2}, weights, "weight should not be allocated to single node")
	require.True(t, weights[len(weights)-1] <= 1, "no node should have weight > 1")
}
