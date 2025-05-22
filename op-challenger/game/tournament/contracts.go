// Package tournament provides functionality for interacting with tournament-based dispute games
// in the Cartesi DAVE fraud proofs system.
package tournament

import (
	"context"
	"fmt"
	"sync"
	"time"

	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/tournament/bindings"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// TournamentGameContract is a wrapper around the TournamentGame contract
type TournamentGameContract struct {
	contract     *bindings.TournamentGame
	caller       *batching.MultiCaller
	txSender     TxSender
	addr         common.Address
	logger       log.Logger
	bondAmount   *big.Int
	matchCache   map[uint64]Match
	nodeCache    map[uint64]Node
	cacheMutex   sync.RWMutex
	cacheTimeout time.Duration
	lastUpdated  time.Time
}

// NewTournamentGameContract creates a new TournamentGameContract
func NewTournamentGameContract(
	addr common.Address,
	caller *batching.MultiCaller,
	txSender TxSender,
	logger log.Logger,
) *TournamentGameContract {
	contract, _ := bindings.NewTournamentGame(addr, caller)

	// Default bond amount is 0.1 ETH (100000000000000000 wei)
	bondAmount := new(big.Int).Mul(big.NewInt(1e17), big.NewInt(1))

	return &TournamentGameContract{
		contract:     contract,
		caller:       caller,
		txSender:     txSender,
		addr:         addr,
		logger:       logger.New("contract", addr),
		bondAmount:   bondAmount,
		matchCache:   make(map[uint64]Match),
		nodeCache:    make(map[uint64]Node),
		cacheTimeout: 5 * time.Minute,
		lastUpdated:  time.Now(),
	}
}

// SetBondAmount sets the bond amount required to join the tournament
func (t *TournamentGameContract) SetBondAmount(amount *big.Int) {
	t.bondAmount = amount
}

// GetStatus returns the current status of the tournament game
func (t *TournamentGameContract) GetStatus(ctx context.Context) (types.GameStatus, error) {
	status, err := t.contract.Status(&bind.CallOpts{Context: ctx})
	if err != nil {
		return types.GameStatusInProgress, fmt.Errorf("failed to get status: %w", err)
	}
	return types.GameStatusFromUint8(uint8(status))
}

// GetTotalParticipants returns the total number of participants in the tournament
func (t *TournamentGameContract) GetTotalParticipants(ctx context.Context) (uint64, error) {
	participants, err := t.contract.TotalParticipants(&bind.CallOpts{Context: ctx})
	if err != nil {
		return 0, fmt.Errorf("failed to get total participants: %w", err)
	}
	return participants.Uint64(), nil
}

// GetCurrentRound returns the current round of the tournament
func (t *TournamentGameContract) GetCurrentRound(ctx context.Context) (uint64, error) {
	round, err := t.contract.CurrentRound(&bind.CallOpts{Context: ctx})
	if err != nil {
		return 0, fmt.Errorf("failed to get current round: %w", err)
	}
	return round.Uint64(), nil
}

// GetMatchCount returns the total number of matches in the tournament
func (t *TournamentGameContract) GetMatchCount(ctx context.Context) (uint64, error) {
	// Check if we need to refresh the cache
	if time.Since(t.lastUpdated) > t.cacheTimeout {
		if err := t.refreshCache(ctx); err != nil {
			t.logger.Warn("Failed to refresh cache", "err", err)
		}
	}

	// Get the match count from the contract
	// This is a custom method that counts the number of matches in the contract
	// by iterating through the matches array until we find an empty match
	count := uint64(0)
	for {
		_, err := t.contract.Matches(&bind.CallOpts{Context: ctx}, big.NewInt(int64(count)))
		if err != nil {
			break
		}
		count++
	}

	return count, nil
}

// HasParticipated checks if an address has participated in the tournament
func (t *TournamentGameContract) HasParticipated(ctx context.Context, addr common.Address) (bool, error) {
	hasParticipated, err := t.contract.HasParticipated(&bind.CallOpts{Context: ctx}, addr)
	if err != nil {
		return false, fmt.Errorf("failed to check if address has participated: %w", err)
	}
	return hasParticipated, nil
}

// JoinTournament joins the tournament with a counter-claim
func (t *TournamentGameContract) JoinTournament(ctx context.Context, claim common.Hash) error {
	// Create the transaction data
	data, err := t.contract.ABI.Pack("joinTournament", claim)
	if err != nil {
		return fmt.Errorf("failed to pack joinTournament call: %w", err)
	}

	// Send the transaction
	tx := TxCandidate{
		TxData:   data,
		To:       &t.addr,
		Value:    t.bondAmount,
		GasLimit: 500000, // Adjust as needed
	}

	t.logger.Info("Joining tournament", "claim", claim.Hex(), "bond", t.bondAmount.String())
	if err := t.txSender.SendAndWaitSimple("joinTournament", tx); err != nil {
		return fmt.Errorf("failed to send joinTournament transaction: %w", err)
	}

	// Invalidate the cache
	t.cacheMutex.Lock()
	t.lastUpdated = time.Time{}
	t.cacheMutex.Unlock()

	return nil
}

// ResolveMatch resolves a match in the tournament
func (t *TournamentGameContract) ResolveMatch(ctx context.Context, matchIndex uint64, winnerIndex uint64) error {
	// Create the transaction data
	data, err := t.contract.ABI.Pack("resolveMatch", big.NewInt(int64(matchIndex)), big.NewInt(int64(winnerIndex)))
	if err != nil {
		return fmt.Errorf("failed to pack resolveMatch call: %w", err)
	}

	// Send the transaction
	tx := TxCandidate{
		TxData:   data,
		To:       &t.addr,
		GasLimit: 500000, // Adjust as needed
	}

	t.logger.Info("Resolving match", "matchIndex", matchIndex, "winnerIndex", winnerIndex)
	if err := t.txSender.SendAndWaitSimple("resolveMatch", tx); err != nil {
		return fmt.Errorf("failed to send resolveMatch transaction: %w", err)
	}

	// Invalidate the cache
	t.cacheMutex.Lock()
	t.lastUpdated = time.Time{}
	t.cacheMutex.Unlock()

	return nil
}

// ClaimBond claims the bond as the tournament winner
func (t *TournamentGameContract) ClaimBond(ctx context.Context) error {
	// Create the transaction data
	data, err := t.contract.ABI.Pack("claimBond")
	if err != nil {
		return fmt.Errorf("failed to pack claimBond call: %w", err)
	}

	// Send the transaction
	tx := TxCandidate{
		TxData:   data,
		To:       &t.addr,
		GasLimit: 300000, // Adjust as needed
	}

	t.logger.Info("Claiming bond")
	if err := t.txSender.SendAndWaitSimple("claimBond", tx); err != nil {
		return fmt.Errorf("failed to send claimBond transaction: %w", err)
	}

	return nil
}

// GetL1Head returns the L1 head hash at the time the tournament was created
func (t *TournamentGameContract) GetL1Head(ctx context.Context) (common.Hash, error) {
	l1Head, err := t.contract.L1Head(&bind.CallOpts{Context: ctx})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get L1 head: %w", err)
	}
	return l1Head, nil
}

// GetRootClaim returns the root claim of the tournament
func (t *TournamentGameContract) GetRootClaim(ctx context.Context) (common.Hash, error) {
	rootClaim, err := t.contract.RootClaim(&bind.CallOpts{Context: ctx})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get root claim: %w", err)
	}
	return rootClaim, nil
}

// GetMatch returns information about a match in the tournament
func (t *TournamentGameContract) GetMatch(ctx context.Context, matchIndex uint64) (Match, error) {
	// Check the cache first
	t.cacheMutex.RLock()
	cachedMatch, exists := t.matchCache[matchIndex]
	t.cacheMutex.RUnlock()

	if exists && time.Since(t.lastUpdated) <= t.cacheTimeout {
		return cachedMatch, nil
	}

	// Get the match from the contract
	matchData, err := t.contract.Matches(&bind.CallOpts{Context: ctx}, big.NewInt(int64(matchIndex)))
	if err != nil {
		return Match{}, fmt.Errorf("failed to get match: %w", err)
	}

	match := Match{
		NodeA:     matchData.NodeA.Uint64(),
		NodeB:     matchData.NodeB.Uint64(),
		Winner:    matchData.Winner.Uint64(),
		StartTime: matchData.StartTime.Uint64(),
		Resolved:  matchData.Resolved,
	}

	// Update the cache
	t.cacheMutex.Lock()
	t.matchCache[matchIndex] = match
	t.cacheMutex.Unlock()

	return match, nil
}

// GetNode returns information about a node in the tournament tree
func (t *TournamentGameContract) GetNode(ctx context.Context, nodeIndex uint64) (Node, error) {
	// Check the cache first
	t.cacheMutex.RLock()
	cachedNode, exists := t.nodeCache[nodeIndex]
	t.cacheMutex.RUnlock()

	if exists && time.Since(t.lastUpdated) <= t.cacheTimeout {
		return cachedNode, nil
	}

	// Get the node from the contract
	nodeData, err := t.contract.Nodes(&bind.CallOpts{Context: ctx}, big.NewInt(int64(nodeIndex)))
	if err != nil {
		return Node{}, fmt.Errorf("failed to get node: %w", err)
	}

	node := Node{
		Participant: nodeData.Participant,
		Claim:       nodeData.Claim,
		HasJoined:   nodeData.HasJoined,
		BondAmount:  nodeData.BondAmount.Uint64(),
	}

	// Update the cache
	t.cacheMutex.Lock()
	t.nodeCache[nodeIndex] = node
	t.cacheMutex.Unlock()

	return node, nil
}

// refreshCache refreshes the cache of matches and nodes
func (t *TournamentGameContract) refreshCache(ctx context.Context) error {
	t.cacheMutex.Lock()
	defer t.cacheMutex.Unlock()

	// Clear the caches
	t.matchCache = make(map[uint64]Match)
	t.nodeCache = make(map[uint64]Node)

	// Get the match count
	count := uint64(0)
	for {
		matchData, err := t.contract.Matches(&bind.CallOpts{Context: ctx}, big.NewInt(int64(count)))
		if err != nil {
			break
		}

		match := Match{
			NodeA:     matchData.NodeA.Uint64(),
			NodeB:     matchData.NodeB.Uint64(),
			Winner:    matchData.Winner.Uint64(),
			StartTime: matchData.StartTime.Uint64(),
			Resolved:  matchData.Resolved,
		}

		t.matchCache[count] = match
		count++
	}

	// Get the nodes
	for i := uint64(0); i < count*2; i++ {
		nodeData, err := t.contract.Nodes(&bind.CallOpts{Context: ctx}, big.NewInt(int64(i)))
		if err != nil {
			break
		}

		node := Node{
			Participant: nodeData.Participant,
			Claim:       nodeData.Claim,
			HasJoined:   nodeData.HasJoined,
			BondAmount:  nodeData.BondAmount.Uint64(),
		}

		t.nodeCache[i] = node
	}

	t.lastUpdated = time.Now()
	return nil
}
