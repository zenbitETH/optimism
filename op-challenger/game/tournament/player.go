package tournament

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/log"
)

// MatchMonitor monitors matches and prioritizes responses
type MatchMonitor struct {
	logger log.Logger
	cfg    *config.Config
}

// NewMatchMonitor creates a new match monitor
func NewMatchMonitor(ctx context.Context, logger log.Logger, cfg *config.Config) (*MatchMonitor, error) {
	return &MatchMonitor{
		logger: logger,
		cfg:    cfg,
	}, nil
}

// GetHighestPriorityMatch gets the highest priority match from a list of matches
func (m *MatchMonitor) GetHighestPriorityMatch(matches []Match) Match {
	if len(matches) == 0 {
		return Match{}
	}

	// Sort by deadline
	sortedMatches := make([]Match, len(matches))
	copy(sortedMatches, matches)

	sort.Slice(sortedMatches, func(i, j int) bool {
		return sortedMatches[i].Deadline < sortedMatches[j].Deadline
	})

	return sortedMatches[0]
}

// Start starts the match monitor
func (m *MatchMonitor) Start(ctx context.Context) error {
	return nil
}

// Stop stops the match monitor
func (m *MatchMonitor) Stop() {
}

// TournamentPlayer implements the game player interface for tournament-based disputes
type TournamentPlayer struct {
	logger   log.Logger
	metrics  metrics.Metricer
	txSender txmgr.TxManager

	// Tournament-specific components
	tournamentManager *Manager
	matchMonitor      *MatchMonitor

	// Existing components
	traceProvider TraceProvider
	cfg           *config.Config
}

// NewTournamentPlayer creates a new tournament player
func NewTournamentPlayer(
	ctx context.Context,
	logger log.Logger,
	metrics metrics.Metricer,
	txSender txmgr.TxManager,
	traceProvider TraceProvider,
	cfg *config.Config,
) (*TournamentPlayer, error) {
	tournamentManager, err := NewManager(ctx, logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create tournament manager: %w", err)
	}

	matchMonitor, err := NewMatchMonitor(ctx, logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create match monitor: %w", err)
	}

	return &TournamentPlayer{
		logger:            logger,
		metrics:           metrics,
		txSender:          txSender,
		tournamentManager: tournamentManager,
		matchMonitor:      matchMonitor,
		traceProvider:     traceProvider,
		cfg:               cfg,
	}, nil
}

// PlayMove determines and plays the next move in the tournament
func (t *TournamentPlayer) PlayMove(ctx context.Context, game Game) (GameAction, error) {
	// Get the tournament state
	tournamentAddr := game.Addr()
	tournamentState, err := t.tournamentManager.GetTournamentState(ctx, tournamentAddr)
	if err != nil {
		return GameAction{}, fmt.Errorf("failed to get tournament state: %w", err)
	}

	// Get the L2 block number
	l2BlockNumber, err := game.L2BlockNumber()
	if err != nil {
		return GameAction{}, fmt.Errorf("failed to get L2 block number: %w", err)
	}

	// Check if we need to join the tournament
	if !tournamentState.IsParticipant(t.cfg.Address) {
		// Generate trace for the disputed block
		trace, err := t.traceProvider.GetTrace(ctx, l2BlockNumber)
		if err != nil {
			return GameAction{}, fmt.Errorf("failed to get trace: %w", err)
		}

		// Generate our claim
		ourClaim, err := trace.GenerateClaim()
		if err != nil {
			return GameAction{}, fmt.Errorf("failed to generate claim: %w", err)
		}

		// Join the tournament
		return GameAction{
			Type:      ActionTypeJoinTournament,
			Claim:     ourClaim,
			ParentIdx: tournamentState.GetRootNodeIndex(),
		}, nil
	}

	// Check if we need to respond to a match
	activeMatches := tournamentState.GetActiveMatches(t.cfg.Address)
	if len(activeMatches) > 0 {
		// Prioritize matches that are close to deadline
		match := t.matchMonitor.GetHighestPriorityMatch(activeMatches)

		// Generate evidence for the match
		evidence, err := t.generateEvidence(ctx, game, match)
		if err != nil {
			return GameAction{}, fmt.Errorf("failed to generate evidence: %w", err)
		}

		return GameAction{
			Type:     ActionTypeSubmitEvidence,
			MatchIdx: match.Index,
			Evidence: evidence,
		}, nil
	}

	// Check if there are unresolved matches that we can resolve
	unresolvedMatches := tournamentState.GetUnresolvedMatches()
	for _, match := range unresolvedMatches {
		if uint64(time.Now().Unix()) > match.Deadline {
			return GameAction{
				Type:     ActionTypeResolveMatch,
				MatchIdx: match.Index,
			}, nil
		}
	}

	// No action needed
	return GameAction{
		Type: ActionTypeNone,
	}, nil
}

// generateEvidence generates evidence for a match
func (t *TournamentPlayer) generateEvidence(ctx context.Context, game Game, match Match) ([]byte, error) {
	// Get the L2 block number
	l2BlockNumber, err := game.L2BlockNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to get L2 block number: %w", err)
	}

	// Get the trace for the disputed block
	trace, err := t.traceProvider.GetTrace(ctx, l2BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get trace: %w", err)
	}

	// Get the claims
	tournamentState, err := t.tournamentManager.GetTournamentState(ctx, game.Addr())
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament state: %w", err)
	}

	nodeA, exists := tournamentState.GetNode(match.NodeA)
	if !exists {
		return nil, fmt.Errorf("node A not found: %d", match.NodeA)
	}

	nodeB, exists := tournamentState.GetNode(match.NodeB)
	if !exists {
		return nil, fmt.Errorf("node B not found: %d", match.NodeB)
	}

	// Generate evidence based on the match type
	return trace.GenerateEvidence(nodeA.Claim, nodeB.Claim)
}

// Start starts the tournament player
func (t *TournamentPlayer) Start(ctx context.Context) error {
	if err := t.tournamentManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start tournament manager: %w", err)
	}

	if err := t.matchMonitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start match monitor: %w", err)
	}

	return nil
}

// Stop stops the tournament player
func (t *TournamentPlayer) Stop() {
	t.tournamentManager.Stop()
	t.matchMonitor.Stop()
}
