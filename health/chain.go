package health

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/docker"
)

// Hoodi expected constants
const (
	ExpectedHoodiChainID            = 560048
	ExpectedHoodiGenesisTime        = "1742213400"
	ExpectedHoodiGenesisForkVersion = "0x10000910"
)

// CheckELChainID verifies that the execution client returns the correct chain ID.
func CheckELChainID(ctx context.Context) error {
	body := `{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`
	// Typically the container is rocketpool_eth1 and the port is 8545
	out, err := docker.PostJSONRPC(ctx, "rocketpool_eth1", 8545, body)
	if err != nil {
		return fmt.Errorf("failed to query EL chain ID: %w", err)
	}

	var resp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return fmt.Errorf("failed to parse EL response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("EL returned error: %s", resp.Error.Message)
	}

	if resp.Result == "" {
		return fmt.Errorf("EL returned empty result")
	}

	// The chainID is a hex string
	id, err := strconv.ParseInt(resp.Result, 0, 64)
	if err != nil {
		return fmt.Errorf("failed to parse chain ID %q: %w", resp.Result, err)
	}

	if id != ExpectedHoodiChainID {
		return fmt.Errorf("unexpected chain ID: got %d, want %d", id, ExpectedHoodiChainID)
	}

	return nil
}

// CheckCLGenesis verifies that the beacon node reports the correct genesis time and fork version.
func CheckCLGenesis(ctx context.Context) error {
	// Typically the container is rocketpool_eth2 and the port is 5052
	out, err := docker.GetHTTP(ctx, "rocketpool_eth2", 5052, "/eth/v1/beacon/genesis")
	if err != nil {
		return fmt.Errorf("failed to query CL genesis: %w", err)
	}

	var resp struct {
		Data struct {
			GenesisTime        string `json:"genesis_time"`
			GenesisForkVersion string `json:"genesis_fork_version"`
		} `json:"data"`
	}

	if err := json.Unmarshal(out, &resp); err != nil {
		return fmt.Errorf("failed to parse CL response: %w", err)
	}

	if resp.Data.GenesisTime != ExpectedHoodiGenesisTime {
		return fmt.Errorf("unexpected CL genesis time: got %q, want %q", resp.Data.GenesisTime, ExpectedHoodiGenesisTime)
	}

	if resp.Data.GenesisForkVersion != ExpectedHoodiGenesisForkVersion {
		return fmt.Errorf("unexpected CL genesis fork version: got %q, want %q", resp.Data.GenesisForkVersion, ExpectedHoodiGenesisForkVersion)
	}

	return nil
}

// CheckELCLAuth verifies that the CL has an authenticated Engine API connection
// to the EL by querying the CL's syncing status. The el_offline field in the
// response is false only when the CL can reach the EL via JWT-authenticated
// Engine API — proving the JWT secret is shared and functional.
func CheckELCLAuth(ctx context.Context) error {
	out, err := docker.GetHTTP(ctx, "rocketpool_eth2", 5052, "/eth/v1/node/syncing")
	if err != nil {
		return fmt.Errorf("failed to query CL syncing status: %w", err)
	}

	var resp struct {
		Data struct {
			ELOffline bool `json:"el_offline"`
		} `json:"data"`
	}

	if err := json.Unmarshal(out, &resp); err != nil {
		return fmt.Errorf("failed to parse CL syncing response: %w", err)
	}

	if resp.Data.ELOffline {
		return fmt.Errorf("CL reports EL is offline — Engine API authentication may have failed")
	}

	return nil
}
