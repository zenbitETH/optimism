// Package tournament provides functionality for interacting with tournament-based dispute games
// in the Cartesi DAVE fraud proofs system.
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

// Player is responsible for playing tournament games by generating and verifying claims
// and participating in matches. It implements higher-level game playing strategies
// compared to the Agent, which focuses on tournament participation mechanics.
type Player struct {
	logger        log.Logger
	agent         *Agent
	manager       *TournamentManager
	traceProvider TraceProvider
	txSender      TxSender

	// Configuration
	maxConcurrent int
	retryInterval time.Duration

	// State
	mu            sync.Mutex
	activeMatches map[uint64]bool
	gameAddress   common.Address
	lastStrategy  time.Time
}

// PlayerConfig contains configuration parameters for the Player
type PlayerConfig struct {
	// MaxConcurrentMatches is the maximum number of matches to participate in concurrently
	MaxConcurrentMatches int
	// RetryInterval is the interval to retry failed operations
	RetryInterval time.Duration
	// BondAmount is the amount to bond when joining a tournament
	BondAmount *big.Int
	// MatchDeadline is the deadline for matches in seconds
	MatchDeadline uint64
	// MatchEffort is the additional time allowance for complex matches in seconds
	MatchEffort uint64
}

// DefaultPlayerConfig returns the default configuration for the Player
func DefaultPlayerConfig() *PlayerConfig {
	return &PlayerConfig{
		MaxConcurrentMatches: 5,
		RetryInterval:        30 * time.Second,
		BondAmount:           new(big.Int).Mul(big.NewInt(1e17), big.NewInt(1)), // 0.1 ETH
		MatchDeadline:        24 * 60 * 60,                                      // 24 hours
		MatchEffort:          12 * 60 * 60,                                      // 12 hours
	}
}

// NewPlayer creates a new tournament player
func NewPlayer(
	ctx context.Context,
	logger log.Logger,
	agent *Agent,
	manager *TournamentManager,
	traceProvider TraceProvider,
	txSender TxSender,
	gameAddress common.Address,
	config *PlayerConfig,
) *Player {
	if config == nil {
		config = DefaultPlayerConfig()
	}

	// Configure the agent with the provided configuration
	if config.BondAmount != nil {
		agent.SetBondAmount(config.BondAmount)
	}
	if config.MatchDeadline > 0 {
		agent.SetMatchDeadline(config.MatchDeadline)
	}
	if config.MatchEffort > 0 {
		agent.SetMatchEffort(config.MatchEffort)
	}

	return &Player{
		logger:        logger.New("component", "Player", "game", gameAddress),
		agent:         agent,
		manager:       manager,
		traceProvider: traceProvider,
		txSender:      txSender,
		maxConcurrent: config.MaxConcurrentMatches,
		retryInterval: config.RetryInterval,
		activeMatches: make(map[uint64]bool),
		gameAddress:   gameAddress,
	}
}

// Start starts the player's main loop
func (p *Player) Start(ctx context.Context) error {
	p.logger.Info("Starting tournament player", "game", p.gameAddress)

	// Initialize the tournament manager
	if err := p.manager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize tournament manager: %w", err)
	}

	// Main loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Tournament player stopped", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			if err := p.tick(ctx); err != nil {
				p.logger.Error("Error in player tick", "err", err)
				// Continue running despite errors
			}
		}
	}
}

// tick performs a single iteration of the player's main loop
func (p *Player) tick(ctx context.Context) error {
	// First, let the agent act on the tournament
	if err := p.agent.Act(ctx); err != nil {
		return fmt.Errorf("agent action failed: %w", err)
	}

	// Check if the tournament is still in progress
	status, err := p.manager.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tournament status: %w", err)
	}

	if status != uint8(types.GameStatusInProgress) {
		p.logger.Info("Tournament is no longer in progress", "status", status)
		return nil
	}

	// Update active matches
	if err := p.updateActiveMatches(ctx); err != nil {
		p.logger.Error("Failed to update active matches", "err", err)
		// Continue despite errors
	}

	// Apply game playing strategy
	if time.Since(p.lastStrategy) > time.Minute {
		if err := p.applyStrategy(ctx); err != nil {
			p.logger.Error("Failed to apply strategy", "err", err)
			// Continue despite errors
		}
		p.lastStrategy = time.Now()
	}

	return nil
}

// updateActiveMatches updates the list of active matches
func (p *Player) updateActiveMatches(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Get all active matches
	activeMatches, err := p.manager.GetActiveMatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active matches: %w", err)
	}

	// Clear the active matches
	p.activeMatches = make(map[uint64]bool)

	// Add all active matches
	for _, matchIndex := range activeMatches {
		match, err := p.manager.GetMatch(ctx, matchIndex)
		if err != nil {
			p.logger.Error("Failed to get match", "index", matchIndex, "err", err)
			continue
		}

		// Check if we're a participant in this match
		nodeA, err := p.manager.GetNode(ctx, match.NodeA)
		if err != nil {
			p.logger.Error("Failed to get node A", "index", match.NodeA, "err", err)
			continue
		}

		nodeB, err := p.manager.GetNode(ctx, match.NodeB)
		if err != nil {
			p.logger.Error("Failed to get node B", "index", match.NodeB, "err", err)
			continue
		}

		// Only track matches where we're a participant
		if nodeA.Participant == p.txSender.From() || nodeB.Participant == p.txSender.From() {
			p.activeMatches[matchIndex] = true
		}
	}

	p.logger.Debug("Updated active matches", "count", len(p.activeMatches))
	return nil
}

// applyStrategy applies the player's game playing strategy
func (p *Player) applyStrategy(ctx context.Context) error {
	p.logger.Debug("Applying game playing strategy")

	// Get prioritized matches
	prioritizedMatches, err := p.manager.GetPrioritizedMatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to get prioritized matches: %w", err)
	}

	// Focus on the top matches based on our concurrency limit
	focusMatches := make([]uint64, 0, p.maxConcurrent)
	for _, matchIndex := range prioritizedMatches {
		p.mu.Lock()
		isActive := p.activeMatches[matchIndex]
		p.mu.Unlock()

		if isActive {
			focusMatches = append(focusMatches, matchIndex)
			if len(focusMatches) >= p.maxConcurrent {
				break
			}
		}
	}

	// Process each focus match
	for _, matchIndex := range focusMatches {
		if err := p.processMatch(ctx, matchIndex); err != nil {
			p.logger.Error("Failed to process match", "index", matchIndex, "err", err)
			// Continue with other matches despite errors
		}
	}

	return nil
}

// processMatch processes a single match
func (p *Player) processMatch(ctx context.Context, matchIndex uint64) error {
	p.logger.Debug("Processing match", "index", matchIndex)

	// Get the match
	match, err := p.manager.GetMatch(ctx, matchIndex)
	if err != nil {
		return fmt.Errorf("failed to get match: %w", err)
	}

	// Skip resolved matches
	if match.Resolved {
		p.logger.Debug("Match already resolved", "index", matchIndex)
		return nil
	}

	// Get the nodes
	nodeA, err := p.manager.GetNode(ctx, match.NodeA)
	if err != nil {
		return fmt.Errorf("failed to get node A: %w", err)
	}

	nodeB, err := p.manager.GetNode(ctx, match.NodeB)
	if err != nil {
		return fmt.Errorf("failed to get node B: %w", err)
	}

	// Determine our role in the match
	var ourNode, theirNode Node
	var ourIndex, theirIndex uint64

	if nodeA.Participant == p.txSender.From() {
		ourNode = nodeA
		theirNode = nodeB
		ourIndex = match.NodeA
		theirIndex = match.NodeB
	} else if nodeB.Participant == p.txSender.From() {
		ourNode = nodeB
		theirNode = nodeA
		ourIndex = match.NodeB
		theirIndex = match.NodeA
	} else {
		return fmt.Errorf("not a participant in this match")
	}

	// Check if the match is ready to be resolved
	expired, err := p.isMatchReadyToResolve(ctx, matchIndex)
	if err != nil {
		return fmt.Errorf("failed to check if match is ready to resolve: %w", err)
	}

	if expired {
		// Determine the winner
		winnerIndex, err := p.determineWinner(ctx, matchIndex, ourIndex, theirIndex, ourNode.Claim, theirNode.Claim)
		if err != nil {
			return fmt.Errorf("failed to determine winner: %w", err)
		}

		// Resolve the match
		if err := p.manager.ResolveMatch(ctx, matchIndex, winnerIndex); err != nil {
			return fmt.Errorf("failed to resolve match: %w", err)
		}

		p.logger.Info("Resolved match", "index", matchIndex, "winner", winnerIndex)
	} else {
		// Prepare for the match by generating and validating proofs
		if err := p.prepareForMatch(ctx, matchIndex, ourIndex, theirIndex, ourNode.Claim, theirNode.Claim); err != nil {
			p.logger.Warn("Failed to prepare for match", "index", matchIndex, "err", err)
			// Continue despite errors
		}
	}

	return nil
}

// isMatchReadyToResolve checks if a match is ready to be resolved
func (p *Player) isMatchReadyToResolve(ctx context.Context, matchIndex uint64) (bool, error) {
	// Check if the match deadline has passed
	expired, err := p.agent.matchMonitor.IsMatchExpired(ctx, matchIndex)
	if err != nil {
		return false, fmt.Errorf("failed to check if match is expired: %w", err)
	}

	if !expired {
		return false, nil
	}

	// Check if we're considering effort
	_, err = p.agent.matchMonitor.GetMatchDeadline(ctx, matchIndex)
	if err != nil {
		return false, fmt.Errorf("failed to get match deadline: %w", err)
	}

	effortDeadline, err := p.agent.matchMonitor.GetMatchEffortDeadline(ctx, matchIndex)
	if err != nil {
		return false, fmt.Errorf("failed to get match effort deadline: %w", err)
	}

	// If the effort deadline has passed, the match is ready to resolve
	return time.Now().After(effortDeadline), nil
}

// determineWinner determines the winner of a match
func (p *Player) determineWinner(
	ctx context.Context,
	matchIndex uint64,
	ourIndex uint64,
	theirIndex uint64,
	ourClaim common.Hash,
	theirClaim common.Hash,
) (uint64, error) {
	p.logger.Debug("Determining winner for match", "index", matchIndex)

	// Generate proofs for both claims
	ourProof, err := p.traceProvider.GenerateProof(ctx, ourIndex)
	if err != nil {
		p.logger.Error("Failed to generate our proof", "err", err)
		return theirIndex, nil // Default to opponent if we can't generate our proof
	}

	theirProof, err := p.traceProvider.GenerateProof(ctx, theirIndex)
	if err != nil {
		p.logger.Error("Failed to generate their proof", "err", err)
		return ourIndex, nil // Default to us if we can't generate their proof
	}

	// Verify the proofs
	ourValid, err := p.traceProvider.VerifyProof(ctx, ourClaim, ourProof)
	if err != nil {
		p.logger.Error("Failed to verify our proof", "err", err)
		ourValid = false
	}

	theirValid, err := p.traceProvider.VerifyProof(ctx, theirClaim, theirProof)
	if err != nil {
		p.logger.Error("Failed to verify their proof", "err", err)
		theirValid = false
	}

	// Determine the winner based on proof validity
	if ourValid && !theirValid {
		return ourIndex, nil
	} else if !ourValid && theirValid {
		return theirIndex, nil
	} else if !ourValid && !theirValid {
		// If neither proof is valid, default to the defender (node A)
		return matchIndex % 2, nil // Node A is always at even indices
	}

	// If both proofs are valid, we need to determine the winner based on the actual execution
	// This would involve comparing the execution traces and determining which one is correct
	// For now, we'll use a simplified approach based on the architecture
	arch := p.traceProvider.GetArchitecture()

	if arch == "riscv" {
		// For RISC-V, we'll use a more sophisticated approach
		// In a real implementation, this would involve running the execution in the Cartesi Machine
		// and comparing the results

		// For now, we'll use a simplified approach based on the claim values
		if ourClaim[0] < theirClaim[0] {
			return ourIndex, nil
		} else {
			return theirIndex, nil
		}
	} else {
		// For MIPS64, we'll use a simpler approach
		// In a real implementation, this would involve running the execution in Cannon
		// and comparing the results

		// For now, we'll use a simplified approach based on the claim values
		if ourClaim[0] > theirClaim[0] {
			return ourIndex, nil
		} else {
			return theirIndex, nil
		}
	}
}

// prepareForMatch prepares for a match by generating and validating proofs
func (p *Player) prepareForMatch(
	ctx context.Context,
	matchIndex uint64,
	ourIndex uint64,
	theirIndex uint64,
	ourClaim common.Hash,
	theirClaim common.Hash,
) error {
	p.logger.Debug("Preparing for match", "index", matchIndex)

	// Generate our proof
	ourProof, err := p.traceProvider.GenerateProof(ctx, ourIndex)
	if err != nil {
		return fmt.Errorf("failed to generate our proof: %w", err)
	}

	// Verify our proof
	ourValid, err := p.traceProvider.VerifyProof(ctx, ourClaim, ourProof)
	if err != nil {
		return fmt.Errorf("failed to verify our proof: %w", err)
	}

	if !ourValid {
		p.logger.Warn("Our proof is invalid", "index", matchIndex)
		// In a real implementation, we would try to generate a new proof or
		// prepare a fallback strategy
	}

	// Generate their proof
	theirProof, err := p.traceProvider.GenerateProof(ctx, theirIndex)
	if err != nil {
		return fmt.Errorf("failed to generate their proof: %w", err)
	}

	// Verify their proof
	theirValid, err := p.traceProvider.VerifyProof(ctx, theirClaim, theirProof)
	if err != nil {
		return fmt.Errorf("failed to verify their proof: %w", err)
	}

	if theirValid {
		p.logger.Info("Their proof is valid", "index", matchIndex)
		// In a real implementation, we would analyze their proof to find weaknesses
		// or prepare a counter-strategy
	} else {
		p.logger.Info("Their proof is invalid", "index", matchIndex)
		// In a real implementation, we would prepare to challenge their proof
	}

	return nil
}

// GetActiveMatchCount returns the number of active matches
func (p *Player) GetActiveMatchCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.activeMatches)
}

// GetGameAddress returns the address of the tournament game
func (p *Player) GetGameAddress() common.Address {
	return p.gameAddress
}

// GetMetrics returns the current tournament metrics
func (p *Player) GetMetrics() *TournamentMetrics {
	return p.manager.GetMetrics()
}
