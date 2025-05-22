package tournament

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// MatchMonitor is responsible for monitoring and prioritizing matches in tournaments
type MatchMonitor struct {
	logger         log.Logger
	loader         TournamentContract
	traceProvider  TraceProvider
	matchDeadline  uint64
	matchEffort    uint64
	activeMatches  map[uint64]Match
	matchPriority  []uint64
	mu             sync.RWMutex
	lastRefresh    time.Time
	refreshTimeout time.Duration
}

// NewMatchMonitor creates a new match monitor
func NewMatchMonitor(
	logger log.Logger,
	loader TournamentContract,
	traceProvider TraceProvider,
	matchDeadline uint64,
	matchEffort uint64,
) *MatchMonitor {
	if matchDeadline == 0 {
		matchDeadline = 24 * 60 * 60 // Default to 24 hours
	}
	if matchEffort == 0 {
		matchEffort = 12 * 60 * 60 // Default to 12 hours
	}

	return &MatchMonitor{
		logger:         logger.New("component", "MatchMonitor"),
		loader:         loader,
		traceProvider:  traceProvider,
		matchDeadline:  matchDeadline,
		matchEffort:    matchEffort,
		activeMatches:  make(map[uint64]Match),
		matchPriority:  []uint64{},
		refreshTimeout: 5 * time.Minute,
	}
}

// RefreshMatches refreshes the list of active matches
func (m *MatchMonitor) RefreshMatches(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we need to refresh
	if time.Since(m.lastRefresh) < m.refreshTimeout {
		return nil
	}

	m.logger.Debug("Refreshing active matches")

	// Get the total number of matches
	matchCount, err := m.loader.GetMatchCount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get match count: %w", err)
	}

	// Clear the active matches
	m.activeMatches = make(map[uint64]Match)
	m.matchPriority = []uint64{}

	// Get all matches
	for i := uint64(0); i < matchCount; i++ {
		match, err := m.loader.GetMatch(ctx, i)
		if err != nil {
			m.logger.Error("Failed to get match", "index", i, "err", err)
			continue
		}

		// Skip resolved matches
		if match.Resolved {
			continue
		}

		// Add to active matches
		m.activeMatches[i] = match
		m.matchPriority = append(m.matchPriority, i)
	}

	// Sort matches by priority
	m.prioritizeMatches(ctx)

	m.lastRefresh = time.Now()
	m.logger.Info("Refreshed active matches", "count", len(m.activeMatches))
	return nil
}

// GetActiveMatches returns the list of active matches
func (m *MatchMonitor) GetActiveMatches(ctx context.Context) (map[uint64]Match, error) {
	if err := m.RefreshMatches(ctx); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy of the active matches
	result := make(map[uint64]Match, len(m.activeMatches))
	for k, v := range m.activeMatches {
		result[k] = v
	}

	return result, nil
}

// GetPrioritizedMatches returns the list of active matches in priority order
func (m *MatchMonitor) GetPrioritizedMatches(ctx context.Context) ([]uint64, error) {
	if err := m.RefreshMatches(ctx); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy of the priority list
	result := make([]uint64, len(m.matchPriority))
	copy(result, m.matchPriority)

	return result, nil
}

// GetMatchDeadline returns the deadline for a match
func (m *MatchMonitor) GetMatchDeadline(ctx context.Context, matchIndex uint64) (time.Time, error) {
	m.mu.RLock()
	match, exists := m.activeMatches[matchIndex]
	m.mu.RUnlock()

	if !exists {
		// Try to get the match from the contract
		var err error
		match, err = m.loader.GetMatch(ctx, matchIndex)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to get match: %w", err)
		}
	}

	// Calculate the deadline
	deadline := time.Unix(int64(match.StartTime+m.matchDeadline), 0)
	return deadline, nil
}

// IsMatchExpired checks if a match has expired
func (m *MatchMonitor) IsMatchExpired(ctx context.Context, matchIndex uint64) (bool, error) {
	deadline, err := m.GetMatchDeadline(ctx, matchIndex)
	if err != nil {
		return false, err
	}

	return time.Now().After(deadline), nil
}

// GetMatchComplexity returns the complexity of a match
func (m *MatchMonitor) GetMatchComplexity(ctx context.Context, matchIndex uint64) (uint64, error) {
	m.mu.RLock()
	match, exists := m.activeMatches[matchIndex]
	m.mu.RUnlock()

	if !exists {
		// Try to get the match from the contract
		var err error
		match, err = m.loader.GetMatch(ctx, matchIndex)
		if err != nil {
			return 0, fmt.Errorf("failed to get match: %w", err)
		}
	}

	// Get the nodes
	nodeA, err := m.loader.GetNode(ctx, match.NodeA)
	if err != nil {
		return 0, fmt.Errorf("failed to get node A: %w", err)
	}

	nodeB, err := m.loader.GetNode(ctx, match.NodeB)
	if err != nil {
		return 0, fmt.Errorf("failed to get node B: %w", err)
	}

	// Calculate complexity based on the difference between the claims
	// This is a simplified approach - in a real implementation, you would
	// need to analyze the actual execution traces to determine complexity
	complexity := calculateClaimDifference(nodeA.Claim, nodeB.Claim)
	return complexity, nil
}

// GetMatchEffortDeadline returns the effort-adjusted deadline for a match
func (m *MatchMonitor) GetMatchEffortDeadline(ctx context.Context, matchIndex uint64) (time.Time, error) {
	deadline, err := m.GetMatchDeadline(ctx, matchIndex)
	if err != nil {
		return time.Time{}, err
	}

	complexity, err := m.GetMatchComplexity(ctx, matchIndex)
	if err != nil {
		return time.Time{}, err
	}

	// Calculate the effort-adjusted deadline
	// The more complex the match, the more time is allowed
	effortFactor := float64(complexity) / 100.0
	if effortFactor > 1.0 {
		effortFactor = 1.0
	}

	additionalTime := time.Duration(float64(m.matchEffort) * effortFactor * float64(time.Second))
	effortDeadline := deadline.Add(additionalTime)

	return effortDeadline, nil
}

// prioritizeMatches sorts the matches by priority
func (m *MatchMonitor) prioritizeMatches(ctx context.Context) {
	// Create a slice of match indices with their priorities
	type matchPriority struct {
		index    uint64
		priority float64
	}

	priorities := make([]matchPriority, 0, len(m.activeMatches))

	for idx, match := range m.activeMatches {
		// Calculate the time until deadline
		deadline := time.Unix(int64(match.StartTime+m.matchDeadline), 0)
		timeUntilDeadline := deadline.Sub(time.Now())

		// Get the match complexity
		complexity, err := m.GetMatchComplexity(ctx, idx)
		if err != nil {
			m.logger.Error("Failed to get match complexity", "index", idx, "err", err)
			complexity = 50 // Default to medium complexity
		}

		// Calculate priority based on time until deadline and complexity
		// Higher priority for matches that are:
		// 1. Closer to deadline
		// 2. Less complex (can be resolved faster)
		timeWeight := 1.0
		if timeUntilDeadline > 0 {
			timeWeight = float64(m.matchDeadline) / float64(timeUntilDeadline.Seconds())
		} else {
			timeWeight = 100.0 // Very high priority for expired matches
		}

		complexityWeight := 100.0 / float64(complexity+1)
		priority := timeWeight * complexityWeight

		priorities = append(priorities, matchPriority{
			index:    idx,
			priority: priority,
		})
	}

	// Sort by priority (higher priority first)
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].priority > priorities[j].priority
	})

	// Update the priority list
	m.matchPriority = make([]uint64, len(priorities))
	for i, p := range priorities {
		m.matchPriority[i] = p.index
	}
}

// calculateClaimDifference calculates the difference between two claims
// This is a simplified approach - in a real implementation, you would
// need to analyze the actual execution traces to determine complexity
func calculateClaimDifference(claimA, claimB common.Hash) uint64 {
	// Count the number of different bytes
	bytesA := claimA.Bytes()
	bytesB := claimB.Bytes()

	diff := uint64(0)
	for i := 0; i < len(bytesA) && i < len(bytesB); i++ {
		if bytesA[i] != bytesB[i] {
			diff++
		}
	}

	return diff * 10 // Scale up for better granularity
}
