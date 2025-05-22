package tournament

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// TournamentManager manages tournament state and interactions
type TournamentManager struct {
	logger        log.Logger
	loader        TournamentContract
	traceProvider TraceProvider
	matchMonitor  *MatchMonitor
	txSender      TxSender

	// Tournament state
	gameAddress common.Address
	rootClaim   common.Hash
	l1Head      common.Hash
	status      uint8

	// Caching
	nodeCache     map[uint64]Node
	matchCache    map[uint64]Match
	cacheMutex    sync.RWMutex
	lastRefresh   time.Time
	refreshPeriod time.Duration

	// Metrics
	metrics *TournamentMetrics
}

// NewTournamentManager creates a new tournament manager
func NewTournamentManager(
	logger log.Logger,
	loader TournamentContract,
	traceProvider TraceProvider,
	matchMonitor *MatchMonitor,
	txSender TxSender,
	gameAddress common.Address,
) *TournamentManager {
	return &TournamentManager{
		logger:        logger.New("component", "TournamentManager", "game", gameAddress),
		loader:        loader,
		traceProvider: traceProvider,
		matchMonitor:  matchMonitor,
		txSender:      txSender,
		gameAddress:   gameAddress,
		nodeCache:     make(map[uint64]Node),
		matchCache:    make(map[uint64]Match),
		refreshPeriod: 2 * time.Minute,
		metrics:       &TournamentMetrics{},
	}
}

// Initialize initializes the tournament manager
func (t *TournamentManager) Initialize(ctx context.Context) error {
	t.logger.Info("Initializing tournament manager")

	// Get the root claim
	rootClaim, err := t.loader.GetRootClaim(ctx)
	if err != nil {
		return fmt.Errorf("failed to get root claim: %w", err)
	}
	t.rootClaim = rootClaim

	// Get the L1 head
	l1Head, err := t.loader.GetL1Head(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L1 head: %w", err)
	}
	t.l1Head = l1Head

	// Get the status
	status, err := t.loader.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
	t.status = uint8(status)

	// Refresh the state
	if err := t.RefreshState(ctx); err != nil {
		t.logger.Warn("Failed to refresh state during initialization", "err", err)
	}

	t.logger.Info("Tournament manager initialized",
		"rootClaim", rootClaim.Hex(),
		"l1Head", l1Head.Hex(),
		"status", status)

	return nil
}

// RefreshState refreshes the tournament state
func (t *TournamentManager) RefreshState(ctx context.Context) error {
	// Check if we need to refresh
	t.cacheMutex.RLock()
	needsRefresh := time.Since(t.lastRefresh) > t.refreshPeriod
	t.cacheMutex.RUnlock()

	if !needsRefresh {
		return nil
	}

	t.logger.Debug("Refreshing tournament state")

	// Get the status
	status, err := t.loader.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
	t.status = uint8(status)

	// Get the total participants
	participants, err := t.loader.GetTotalParticipants(ctx)
	if err != nil {
		return fmt.Errorf("failed to get total participants: %w", err)
	}

	// Get the current round
	round, err := t.loader.GetCurrentRound(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current round: %w", err)
	}

	// Get the match count
	matchCount, err := t.loader.GetMatchCount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get match count: %w", err)
	}

	// Count resolved matches
	resolvedMatches := uint64(0)
	activeMatches := uint64(0)

	// Clear the caches
	t.cacheMutex.Lock()
	t.nodeCache = make(map[uint64]Node)
	t.matchCache = make(map[uint64]Match)

	// Refresh match cache
	for i := uint64(0); i < matchCount; i++ {
		match, err := t.loader.GetMatch(ctx, i)
		if err != nil {
			t.logger.Error("Failed to get match", "index", i, "err", err)
			continue
		}

		t.matchCache[i] = match

		if match.Resolved {
			resolvedMatches++
		} else {
			activeMatches++
		}

		// Cache the nodes for this match
		if _, exists := t.nodeCache[match.NodeA]; !exists {
			nodeA, err := t.loader.GetNode(ctx, match.NodeA)
			if err != nil {
				t.logger.Error("Failed to get node", "index", match.NodeA, "err", err)
			} else {
				t.nodeCache[match.NodeA] = nodeA
			}
		}

		if _, exists := t.nodeCache[match.NodeB]; !exists {
			nodeB, err := t.loader.GetNode(ctx, match.NodeB)
			if err != nil {
				t.logger.Error("Failed to get node", "index", match.NodeB, "err", err)
			} else {
				t.nodeCache[match.NodeB] = nodeB
			}
		}
	}

	// Update metrics
	t.metrics.TotalMatches = matchCount
	t.metrics.ResolvedMatches = resolvedMatches
	t.metrics.ActiveMatches = activeMatches
	t.metrics.TotalParticipants = participants
	t.metrics.CurrentRound = round
	t.metrics.LastUpdateTime = time.Now()

	t.lastRefresh = time.Now()
	t.cacheMutex.Unlock()

	t.logger.Info("Tournament state refreshed",
		"status", status,
		"participants", participants,
		"round", round,
		"matches", matchCount,
		"resolved", resolvedMatches,
		"active", activeMatches)

	return nil
}

// GetStatus returns the current status of the tournament
func (t *TournamentManager) GetStatus(ctx context.Context) (uint8, error) {
	if err := t.RefreshState(ctx); err != nil {
		return t.status, err
	}
	return t.status, nil
}

// IsComplete checks if the tournament is complete
func (t *TournamentManager) IsComplete(ctx context.Context) (bool, error) {
	status, err := t.GetStatus(ctx)
	if err != nil {
		return false, err
	}
	return status != 0, nil // 0 = IN_PROGRESS
}

// GetNode returns information about a node in the tournament tree
func (t *TournamentManager) GetNode(ctx context.Context, nodeIndex uint64) (Node, error) {
	// Check the cache first
	t.cacheMutex.RLock()
	node, exists := t.nodeCache[nodeIndex]
	t.cacheMutex.RUnlock()

	if exists {
		return node, nil
	}

	// Get from the contract
	node, err := t.loader.GetNode(ctx, nodeIndex)
	if err != nil {
		return Node{}, err
	}

	// Update the cache
	t.cacheMutex.Lock()
	t.nodeCache[nodeIndex] = node
	t.cacheMutex.Unlock()

	return node, nil
}

// GetMatch returns information about a match in the tournament
func (t *TournamentManager) GetMatch(ctx context.Context, matchIndex uint64) (Match, error) {
	// Check the cache first
	t.cacheMutex.RLock()
	match, exists := t.matchCache[matchIndex]
	t.cacheMutex.RUnlock()

	if exists {
		return match, nil
	}

	// Get from the contract
	match, err := t.loader.GetMatch(ctx, matchIndex)
	if err != nil {
		return Match{}, err
	}

	// Update the cache
	t.cacheMutex.Lock()
	t.matchCache[matchIndex] = match
	t.cacheMutex.Unlock()

	return match, nil
}

// GetActiveMatches returns the list of active matches
func (t *TournamentManager) GetActiveMatches(ctx context.Context) ([]uint64, error) {
	if err := t.RefreshState(ctx); err != nil {
		return nil, err
	}

	t.cacheMutex.RLock()
	defer t.cacheMutex.RUnlock()

	activeMatches := make([]uint64, 0, t.metrics.ActiveMatches)
	for idx, match := range t.matchCache {
		if !match.Resolved {
			activeMatches = append(activeMatches, idx)
		}
	}

	return activeMatches, nil
}

// GetPrioritizedMatches returns the list of active matches in priority order
func (t *TournamentManager) GetPrioritizedMatches(ctx context.Context) ([]uint64, error) {
	return t.matchMonitor.GetPrioritizedMatches(ctx)
}

// JoinTournament joins the tournament with a counter-claim
func (t *TournamentManager) JoinTournament(ctx context.Context, claim common.Hash) error {
	t.logger.Info("Joining tournament", "claim", claim.Hex())

	// Check if we've already participated
	hasParticipated, err := t.loader.HasParticipated(ctx, t.txSender.From())
	if err != nil {
		return fmt.Errorf("failed to check if we've participated: %w", err)
	}

	if hasParticipated {
		t.logger.Info("Already participated in tournament")
		return nil
	}

	// Join the tournament
	if err := t.loader.JoinTournament(ctx, claim); err != nil {
		return fmt.Errorf("failed to join tournament: %w", err)
	}

	// Refresh the state
	t.cacheMutex.Lock()
	t.lastRefresh = time.Time{} // Force refresh
	t.cacheMutex.Unlock()

	if err := t.RefreshState(ctx); err != nil {
		t.logger.Warn("Failed to refresh state after joining tournament", "err", err)
	}

	t.logger.Info("Successfully joined tournament")
	return nil
}

// ResolveMatch resolves a match in the tournament
func (t *TournamentManager) ResolveMatch(ctx context.Context, matchIndex uint64, winnerIndex uint64) error {
	t.logger.Info("Resolving match", "matchIndex", matchIndex, "winnerIndex", winnerIndex)

	// Get the match
	match, err := t.GetMatch(ctx, matchIndex)
	if err != nil {
		return fmt.Errorf("failed to get match: %w", err)
	}

	// Check if the match is already resolved
	if match.Resolved {
		t.logger.Info("Match already resolved", "matchIndex", matchIndex)
		return nil
	}

	// Check if we're a participant in this match
	nodeA, err := t.GetNode(ctx, match.NodeA)
	if err != nil {
		return fmt.Errorf("failed to get node A: %w", err)
	}

	nodeB, err := t.GetNode(ctx, match.NodeB)
	if err != nil {
		return fmt.Errorf("failed to get node B: %w", err)
	}

	if nodeA.Participant != t.txSender.From() && nodeB.Participant != t.txSender.From() {
		return fmt.Errorf("not a participant in this match")
	}

	// Check if the match deadline has passed
	deadline, err := t.matchMonitor.GetMatchDeadline(ctx, matchIndex)
	if err != nil {
		return fmt.Errorf("failed to get match deadline: %w", err)
	}

	if time.Now().Before(deadline) {
		// Check if we're considering effort
		effortDeadline, err := t.matchMonitor.GetMatchEffortDeadline(ctx, matchIndex)
		if err != nil {
			return fmt.Errorf("failed to get match effort deadline: %w", err)
		}

		if time.Now().Before(effortDeadline) {
			return fmt.Errorf("match deadline has not passed yet (with effort): %v", effortDeadline)
		}
	}

	// Resolve the match
	if err := t.loader.ResolveMatch(ctx, matchIndex, winnerIndex); err != nil {
		return fmt.Errorf("failed to resolve match: %w", err)
	}

	// Refresh the state
	t.cacheMutex.Lock()
	t.lastRefresh = time.Time{} // Force refresh
	t.cacheMutex.Unlock()

	if err := t.RefreshState(ctx); err != nil {
		t.logger.Warn("Failed to refresh state after resolving match", "err", err)
	}

	t.logger.Info("Successfully resolved match", "matchIndex", matchIndex, "winnerIndex", winnerIndex)
	return nil
}

// ClaimBond claims the bond as the tournament winner
func (t *TournamentManager) ClaimBond(ctx context.Context) error {
	t.logger.Info("Claiming bond")

	// Check if the tournament is complete
	isComplete, err := t.IsComplete(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if tournament is complete: %w", err)
	}

	if !isComplete {
		return fmt.Errorf("tournament is not complete")
	}

	// Claim the bond
	if err := t.loader.ClaimBond(ctx); err != nil {
		return fmt.Errorf("failed to claim bond: %w", err)
	}

	t.logger.Info("Successfully claimed bond")
	return nil
}

// GetMetrics returns the current tournament metrics
func (t *TournamentManager) GetMetrics() *TournamentMetrics {
	t.cacheMutex.RLock()
	defer t.cacheMutex.RUnlock()

	// Create a copy of the metrics
	metrics := &TournamentMetrics{
		TotalMatches:      t.metrics.TotalMatches,
		ResolvedMatches:   t.metrics.ResolvedMatches,
		ActiveMatches:     t.metrics.ActiveMatches,
		TotalParticipants: t.metrics.TotalParticipants,
		CurrentRound:      t.metrics.CurrentRound,
		LastUpdateTime:    t.metrics.LastUpdateTime,
	}

	return metrics
}

// DetermineWinner determines the winner of a match based on trace evidence
func (t *TournamentManager) DetermineWinner(ctx context.Context, matchIndex uint64) (uint64, error) {
	t.logger.Debug("Determining winner for match", "matchIndex", matchIndex)

	// Get the match
	match, err := t.GetMatch(ctx, matchIndex)
	if err != nil {
		return 0, fmt.Errorf("failed to get match: %w", err)
	}

	// Get the nodes
	nodeA, err := t.GetNode(ctx, match.NodeA)
	if err != nil {
		return 0, fmt.Errorf("failed to get node A: %w", err)
	}

	nodeB, err := t.GetNode(ctx, match.NodeB)
	if err != nil {
		return 0, fmt.Errorf("failed to get node B: %w", err)
	}

	// Generate proofs for both claims
	proofA, err := t.traceProvider.GenerateProof(ctx, match.NodeA)
	if err != nil {
		t.logger.Error("Failed to generate proof for node A", "err", err)
		return match.NodeB, nil // Default to node B if we can't generate proof for node A
	}

	proofB, err := t.traceProvider.GenerateProof(ctx, match.NodeB)
	if err != nil {
		t.logger.Error("Failed to generate proof for node B", "err", err)
		return match.NodeA, nil // Default to node A if we can't generate proof for node B
	}

	// Verify the proofs
	validA, err := t.traceProvider.VerifyProof(ctx, nodeA.Claim, proofA)
	if err != nil {
		t.logger.Error("Failed to verify proof for node A", "err", err)
		validA = false
	}

	validB, err := t.traceProvider.VerifyProof(ctx, nodeB.Claim, proofB)
	if err != nil {
		t.logger.Error("Failed to verify proof for node B", "err", err)
		validB = false
	}

	// Determine the winner based on proof validity
	if validA && !validB {
		return match.NodeA, nil
	} else if !validA && validB {
		return match.NodeB, nil
	} else if !validA && !validB {
		// If neither proof is valid, default to the defender (node A)
		return match.NodeA, nil
	}

	// If both proofs are valid, we need to determine the winner based on the actual execution
	// This would involve comparing the execution traces and determining which one is correct
	// For now, we'll use a simplified approach based on the architecture
	arch := t.traceProvider.GetArchitecture()

	if arch == "riscv" {
		// For RISC-V, we'll use a more sophisticated approach
		// In a real implementation, this would involve running the execution in the Cartesi Machine
		// and comparing the results

		// For now, we'll use a simplified approach based on the claim values
		if nodeA.Claim[0] < nodeB.Claim[0] {
			return match.NodeA, nil
		} else {
			return match.NodeB, nil
		}
	} else {
		// For MIPS64, we'll use a simpler approach
		// In a real implementation, this would involve running the execution in Cannon
		// and comparing the results

		// For now, we'll use a simplified approach based on the claim values
		if nodeA.Claim[0] > nodeB.Claim[0] {
			return match.NodeA, nil
		} else {
			return match.NodeB, nil
		}
	}
}
