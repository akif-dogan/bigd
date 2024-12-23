package siatest

import (
	"path/filepath"
	"testing"

	"go.thebigfile.com/bigd/build"
	"go.thebigfile.com/bigd/node"
)

// TestNewGroup tests the behavior of NewGroup.
func TestNewGroup(t *testing.T) {
	if !build.VLONG {
		t.SkipNow()
	}
	t.Parallel()

	// Specify the parameters for the group
	groupParams := GroupParams{
		Hosts:   5,
		Renters: 2,
		Miners:  2,
	}
	// Create the group
	tg, err := NewGroupFromTemplate(siatestTestDir(t.Name()), groupParams)
	if err != nil {
		t.Fatal("Failed to create group: ", err)
	}
	defer func() {
		if err := tg.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	// Check if the correct number of nodes was created
	if len(tg.Hosts()) != groupParams.Hosts {
		t.Error("Wrong number of hosts")
	}
	expectedRenters := groupParams.Renters
	if len(tg.Renters()) != expectedRenters {
		t.Error("Wrong number of renters")
	}
	if len(tg.Miners()) != groupParams.Miners {
		t.Error("Wrong number of miners")
	}
	expectedNumberNodes := groupParams.Hosts + groupParams.Renters + groupParams.Miners
	if len(tg.Nodes()) != expectedNumberNodes {
		t.Error("Wrong number of nodes")
	}

	// Check that all hosts are announced and have a registry.
	for _, host := range tg.Hosts() {
		hg, err := host.HostGet()
		if err != nil {
			t.Fatal(err)
		}
		if !hg.InternalSettings.AcceptingContracts {
			t.Fatal("host not accepting contracts")
		}
		if hg.InternalSettings.RegistrySize == 0 {
			t.Fatal("registry not set")
		}
	}

	// Check if nodes are funded
	cg, err := tg.Nodes()[0].ConsensusGet()
	if err != nil {
		t.Fatal("Failed to get consensus: ", err)
	}
	for _, node := range tg.Nodes() {
		wtg, err := node.WalletTransactionsGet(0, cg.Height)
		if err != nil {
			t.Fatal(err)
		}
		if len(wtg.ConfirmedTransactions) == 0 {
			t.Errorf("Node has 0 confirmed funds")
		}
	}
}

// TestNewGroupNoMiner tests NewGroup without a miner
func TestNewGroupNoMiner(t *testing.T) {
	if !build.VLONG {
		t.SkipNow()
	}
	t.Parallel()

	// Try to create a group without miners
	groupParams := GroupParams{
		Hosts:   5,
		Renters: 2,
		Miners:  0,
	}
	// Create the group
	_, err := NewGroupFromTemplate(siatestTestDir(t.Name()), groupParams)
	if err == nil {
		t.Fatal("Creating a group without miners should fail: ", err)
	}
}

// TestNewGroupNoRenterHost tests NewGroup with no renter or host
func TestNewGroupNoRenterHost(t *testing.T) {
	if !build.VLONG {
		t.SkipNow()
	}
	t.Parallel()

	// Create a group with nothing but miners
	groupParams := GroupParams{
		Hosts:   0,
		Renters: 0,
		Miners:  5,
	}
	// Create the group
	tg, err := NewGroupFromTemplate(siatestTestDir(t.Name()), groupParams)
	if err != nil {
		t.Fatal("Failed to create group: ", err)
	}
	func() {
		if err := tg.Close(); err != nil {
			t.Fatal(err)
		}
	}()
}

// TestAddNewNode tests that the added node is returned when AddNodes is called
func TestAddNewNode(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create a group
	groupParams := GroupParams{
		Renters: 2,
		Miners:  1,
	}
	groupDir := siatestTestDir(t.Name())
	tg, err := NewGroupFromTemplate(groupDir, groupParams)
	if err != nil {
		t.Fatal("Failed to create group: ", err)
	}
	defer func() {
		if err := tg.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	// Record current nodes
	oldRenters := tg.Renters()

	// Test adding a node
	renterTemplate := node.Renter(filepath.Join(groupDir, "renter"))
	nodes, err := tg.AddNodes(renterTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("More nodes returned than expected; expected 1 got %v", len(nodes))
	}
	renter := nodes[0]
	for _, oldRenter := range oldRenters {
		if oldRenter.primarySeed == renter.primarySeed {
			t.Fatal("Returned renter is not the new renter")
		}
	}
}
