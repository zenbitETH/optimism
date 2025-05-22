package tournament

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/tournament/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// Manager manages tournament state and interactions
type Manager struct {
	logger log.Logger
	cfg    *config.Config
	client *contracts.TournamentClient

	tournaments map[common.Address]*TournamentState
	mu          sync.RWMutex

	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// NewManager creates a new tournament manager
func NewManager(ctx context.Context, logger log.Logger, cfg *config.Config) (*Manager, error) {
	client, err := contracts.NewTournamentClient(ctx, logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create tournament client: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	return &Manager{
		logger:      logger,
		cfg:         cfg,
		client:      client,
		tournaments: make(map[common.Address]*TournamentState),
		ctx:         ctx,
		cancelFunc:  cancel,
	}, nil
}

// GetTournamentState gets the state of a tournament
func (m *Manager) GetTournamentState(ctx context.Context, tournamentAddr common.Address) (*TournamentState, error) {
	m.mu.RLock()
	state, exists := m.tournaments[tournamentAddr]
	m.mu.RUnlock()

	if exists {
		return state, nil
	}

	// Fetch tournament state from the contract
	state, err := m.fetchTournamentState(ctx, tournamentAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tournament state: %w", err)
	}

	m.mu.Lock()
	m.tournaments[tournamentAddr] = state
	m.mu.Unlock()

	return state, nil
}

// fetchTournamentState fetches the state of a tournament from the contract
func (m *Manager) fetchTournamentState(ctx context.Context, tournamentAddr common.Address) (*TournamentState, error) {
	// Create tournament contract binding
	tournament, err := m.client.GetTournament(ctx, tournamentAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament contract: %w", err)
	}

	// Fetch nodes
	nodeCount, err := tournament.GetNodeCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node count: %w", err)
	}

	nodes := make([]Node, 0, nodeCount)
	for i := uint64(0); i < nodeCount; i++ {
		node, err := tournament.GetNode(ctx, i)
		if err != nil {
			return nil, fmt.Errorf("failed to get node %d: %w", i, err)
		}
		nodes = append(nodes, Node{
			Index:      i,
			Claim:      node.Claim,
			Claimant:   node.Claimant,
			Timestamp:  node.Timestamp,
			Challenged: node.Challenged,
		})
	}

	// Fetch matches
	matchCount, err := tournament.GetMatchCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get match count: %w", err)
	}

	matches := make([]Match, 0, matchCount)
	for i := uint64(0); i < matchCount; i++ {
		match, err := tournament.GetMatch(ctx, i)
		if err != nil {
			return nil, fmt.Errorf("failed to get match %d: %w", i, err)
		}
		matches = append(matches, Match{
			Index:    i,
			NodeA:    match.NodeIndex1,
			NodeB:    match.NodeIndex2,
			Deadline: match.Deadline,
			Winner:   match.Winner,
			Evidence: match.Evidence,
		})
	}

	return NewTournamentState(tournamentAddr, nodes, matches), nil
}

// Start starts the tournament manager
func (m *Manager) Start(ctx context.Context) error {
	m.wg.Add(1)
	go m.monitorTournaments()
	return nil
}

// Stop stops the tournament manager
func (m *Manager) Stop() {
	m.cancelFunc()
	m.wg.Wait()
}

// monitorTournaments monitors tournaments for updates
func (m *Manager) monitorTournaments() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateTournaments()
		}
	}
}

// updateTournaments updates the state of all tracked tournaments
func (m *Manager) updateTournaments() {
	m.mu.RLock()
	tournaments := make([]common.Address, 0, len(m.tournaments))
	for addr := range m.tournaments {
		tournaments = append(tournaments, addr)
	}
	m.mu.RUnlock()

	for _, addr := range tournaments {
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		state, err := m.fetchTournamentState(ctx, addr)
		cancel()

		if err != nil {
			m.logger.Error("Failed to update tournament state", "tournament", addr, "error", err)
			continue
		}

		m.mu.Lock()
		m.tournaments[addr] = state
		m.mu.Unlock()
	}
}
