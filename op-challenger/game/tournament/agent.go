package tournament

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// Agent is responsible for monitoring tournaments and taking actions
type Agent struct {
	clock         Clock
	loader        TournamentContract
	txSender      TxSender
	traceProvider TraceProvider
	manager       *TournamentManager
	matchMonitor  *MatchMonitor
	logger        log.Logger
	hasJoined     bool

	// Configuration
	bondAmount    *big.Int
	matchDeadline uint64
	matchEffort   uint64

	// State
	mu            sync.Mutex
	lastAction    time.Time
	actionTimeout time.Duration
}

// Clock interface for time-related operations
type Clock interface {
	Now() time.Time
}

// SystemClock implements the Clock interface using the system clock
type SystemClock struct{}

// Now returns the current time
func (s *SystemClock) Now() time.Time {
	return time.Now()
}

// NewAgent creates a new tournament agent
func NewAgent(
	clock Clock,
	loader TournamentContract,
	txSender TxSender,
	traceProvider TraceProvider,
	logger log.Logger,
	hasJoined bool,
) *Agent {
	// Default configuration
	bondAmount := new(big.Int).Mul(big.NewInt(1e17), big.NewInt(1)) // 0.1 ETH
	matchDeadline := uint64(24 * 60 * 60)                           // 24 hours
	matchEffort := uint64(12 * 60 * 60)                             // 12 hours

	// Create match monitor
	matchMonitor := NewMatchMonitor(
		logger.New("component", "MatchMonitor"),
		loader,
		traceProvider,
		matchDeadline,
		matchEffort,
	)

	// Create tournament manager
	manager := NewTournamentManager(
		logger.New("component", "TournamentManager"),
		loader,
		traceProvider,
		matchMonitor,
		txSender,
		common.Address{}, // Will be set during initialization
	)

	return &Agent{
		clock:         clock,
		loader:        loader,
		txSender:      txSender,
		traceProvider: traceProvider,
		manager:       manager,
		matchMonitor:  matchMonitor,
		logger:        logger.New("component", "Agent"),
		hasJoined:     hasJoined,
		bondAmount:    bondAmount,
		matchDeadline: matchDeadline,
		matchEffort:   matchEffort,
		actionTimeout: 5 * time.Minute,
	}
}

// SetBondAmount sets the bond amount required to join the tournament
func (a *Agent) SetBondAmount(amount *big.Int) {
	a.bondAmount = amount
}

// SetMatchDeadline sets the deadline for matches
func (a *Agent) SetMatchDeadline(deadline uint64) {
	a.matchDeadline = deadline
}

// SetMatchEffort sets the additional time allowance for complex matches
func (a *Agent) SetMatchEffort(effort uint64) {
	a.matchEffort = effort
}

// Act performs the necessary actions for the tournament
func (a *Agent) Act(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if we need to wait before acting again
	if time.Since(a.lastAction) < a.actionTimeout {
		return nil
	}

	a.logger.Debug("Acting on tournament")

	// Check if the tournament is still in progress
	status, err := a.loader.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tournament status: %w", err)
	}

	if status != types.GameStatusInProgress {
		a.logger.Info("Tournament is no longer in progress", "status", status)

		// If the tournament is complete, try to claim the bond
		if status == types.GameStatusChallengerWon || status == types.GameStatusDefenderWon {
			if err := a.checkAndClaimBond(ctx); err != nil {
				a.logger.Warn("Failed to claim bond", "err", err)
			}
		}

		return nil
	}

	// If we haven't joined the tournament yet, join it
	if !a.hasJoined {
		if err := a.joinTournament(ctx); err != nil {
			return fmt.Errorf("failed to join tournament: %w", err)
		}
		a.hasJoined = true
	}

	// Check for matches that need to be resolved
	if err := a.checkAndResolveMatches(ctx); err != nil {
		return fmt.Errorf("failed to check and resolve matches: %w", err)
	}

	a.lastAction = a.clock.Now()
	return nil
}

// joinTournament joins the tournament with a counter-claim
func (a *Agent) joinTournament(ctx context.Context) error {
	a.logger.Info("Joining tournament")

	// Get the root claim to generate a counter-claim
	rootClaim, err := a.loader.GetRootClaim(ctx)
	if err != nil {
		return fmt.Errorf("failed to get root claim: %w", err)
	}

	// Generate a counter-claim using the trace provider
	// We'll analyze the root claim to determine the appropriate counter-claim
	counterClaim, err := a.generateCounterClaim(ctx, rootClaim)
	if err != nil {
		return fmt.Errorf("failed to generate counter-claim: %w", err)
	}

	a.logger.Info("Generated counter-claim", "rootClaim", rootClaim.Hex(), "counterClaim", counterClaim.Hex())

	// Join the tournament with our counter-claim
	if err := a.manager.JoinTournament(ctx, counterClaim); err != nil {
		return fmt.Errorf("failed to join tournament: %w", err)
	}

	a.logger.Info("Successfully joined tournament")
	return nil
}

// generateCounterClaim generates a counter-claim based on the root claim
func (a *Agent) generateCounterClaim(ctx context.Context, rootClaim common.Hash) (common.Hash, error) {
	a.logger.Debug("Generating counter-claim", "rootClaim", rootClaim.Hex())

	// In a production implementation, we would:
	// 1. Analyze the root claim to determine what it represents
	// 2. Generate our own execution trace for the same block
	// 3. Find the first point of disagreement
	// 4. Create a counter-claim at that point

	// For now, we'll use a simplified approach:
	// We'll get the trace at index 1, which represents the first step of execution
	counterClaim, err := a.traceProvider.GetTrace(ctx, 1)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get trace at index 1: %w", err)
	}

	// If the counter-claim is the same as the root claim, we need to find a different one
	if counterClaim == rootClaim {
		// Try the next trace index
		counterClaim, err = a.traceProvider.GetTrace(ctx, 2)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to get trace at index 2: %w", err)
		}
	}

	return counterClaim, nil
}

// checkAndResolveMatches checks for matches that need to be resolved and resolves them
func (a *Agent) checkAndResolveMatches(ctx context.Context) error {
	a.logger.Debug("Checking for matches to resolve")

	// Get prioritized matches
	matches, err := a.manager.GetPrioritizedMatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to get prioritized matches: %w", err)
	}

	if len(matches) == 0 {
		a.logger.Debug("No active matches to resolve")
		return nil
	}

	a.logger.Info("Found active matches", "count", len(matches))

	// Process each match in priority order
	for _, matchIndex := range matches {
		match, err := a.manager.GetMatch(ctx, matchIndex)
		if err != nil {
			a.logger.Error("Failed to get match", "index", matchIndex, "err", err)
			continue
		}

		// Skip already resolved matches
		if match.Resolved {
			continue
		}

		// Check if we're a participant in this match
		nodeA, err := a.manager.GetNode(ctx, match.NodeA)
		if err != nil {
			a.logger.Error("Failed to get node A", "index", match.NodeA, "err", err)
			continue
		}

		nodeB, err := a.manager.GetNode(ctx, match.NodeB)
		if err != nil {
			a.logger.Error("Failed to get node B", "index", match.NodeB, "err", err)
			continue
		}

		// Check if we're a participant in this match
		if nodeA.Participant != a.txSender.From() && nodeB.Participant != a.txSender.From() {
			continue
		}

		// Check if the match deadline has passed
		expired, err := a.matchMonitor.IsMatchExpired(ctx, matchIndex)
		if err != nil {
			a.logger.Error("Failed to check if match is expired", "index", matchIndex, "err", err)
			continue
		}

		if !expired {
			deadline, _ := a.matchMonitor.GetMatchDeadline(ctx, matchIndex)
			a.logger.Debug("Match not ready to be resolved yet", "index", matchIndex, "deadline", deadline)
			continue
		}

		// Determine the winner
		winnerIndex, err := a.manager.DetermineWinner(ctx, matchIndex)
		if err != nil {
			a.logger.Error("Failed to determine winner", "match", matchIndex, "err", err)
			continue
		}

		a.logger.Info("Resolving match", "index", matchIndex, "winner", winnerIndex)

		// Resolve the match
		if err := a.manager.loader.ResolveMatch(ctx, matchIndex, winnerIndex); err != nil {
			a.logger.Error("Failed to resolve match", "index", matchIndex, "err", err)
			continue
		}

		a.logger.Info("Successfully resolved match", "index", matchIndex, "winner", winnerIndex)

		// Only resolve one match per action to avoid timeouts
		break
	}

	return nil
}

// checkAndClaimBond checks if we can claim the bond and claims it if possible
func (a *Agent) checkAndClaimBond(ctx context.Context) error {
	a.logger.Debug("Checking if we can claim the bond")

	// Check if the tournament is resolved
	status, err := a.loader.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tournament status: %w", err)
	}

	// If the tournament is not resolved, we can't claim the bond
	if status == types.GameStatusInProgress {
		a.logger.Debug("Tournament is not completed yet", "status", status)
		return nil
	}

	// Try to claim the bond
	if err := a.manager.loader.ClaimBond(ctx); err != nil {
		a.logger.Debug("Failed to claim bond, likely not the winner", "err", err)
		return nil
	}

	a.logger.Info("Successfully claimed bond")
	return nil
}
