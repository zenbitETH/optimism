package tournament

import (
	"sort"

	"github.com/ethereum/go-ethereum/common"
)

// TournamentState represents the state of a tournament
type TournamentState struct {
	address common.Address
	nodes   []Node
	matches []Match
}

// NewTournamentState creates a new tournament state
func NewTournamentState(address common.Address, nodes []Node, matches []Match) *TournamentState {
	return &TournamentState{
		address: address,
		nodes:   nodes,
		matches: matches,
	}
}

// IsParticipant checks if an address is a participant in the tournament
func (t *TournamentState) IsParticipant(address common.Address) bool {
	for _, node := range t.nodes {
		if node.Claimant == address {
			return true
		}
	}
	return false
}

// GetRootNodeIndex gets the index of the root node
func (t *TournamentState) GetRootNodeIndex() uint64 {
	return 0
}

// GetActiveMatches gets active matches for a participant
func (t *TournamentState) GetActiveMatches(address common.Address) []Match {
	activeMatches := make([]Match, 0)

	// Map of node indices owned by the participant
	ownedNodes := make(map[uint64]bool)
	for _, node := range t.nodes {
		if node.Claimant == address {
			ownedNodes[node.Index] = true
		}
	}

	// Find matches where the participant is involved and the match is not resolved
	for _, match := range t.matches {
		if (ownedNodes[match.NodeA] || ownedNodes[match.NodeB]) && match.Winner == 0 {
			activeMatches = append(activeMatches, match)
		}
	}

	return activeMatches
}

// GetNode gets a node by index
func (t *TournamentState) GetNode(index uint64) (Node, bool) {
	if index >= uint64(len(t.nodes)) {
		return Node{}, false
	}
	return t.nodes[index], true
}

// GetMatch gets a match by index
func (t *TournamentState) GetMatch(index uint64) (Match, bool) {
	if index >= uint64(len(t.matches)) {
		return Match{}, false
	}
	return t.matches[index], true
}

// GetUnresolvedMatches gets all unresolved matches in the tournament
func (t *TournamentState) GetUnresolvedMatches() []Match {
	unresolvedMatches := make([]Match, 0)
	for _, match := range t.matches {
		if match.Winner == 0 {
			unresolvedMatches = append(unresolvedMatches, match)
		}
	}
	return unresolvedMatches
}

// GetMatchesByDeadline gets matches ordered by deadline
func (t *TournamentState) GetMatchesByDeadline() []Match {
	// Make a copy of matches
	matches := make([]Match, len(t.matches))
	copy(matches, t.matches)

	// Sort by deadline
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Deadline < matches[j].Deadline
	})

	return matches
}
