package tournament

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Node represents a node in the tournament tree
type Node struct {
	Index      uint64
	Claim      common.Hash
	Claimant   common.Address
	Timestamp  uint64
	Challenged bool
}

// Match represents a match in the tournament
type Match struct {
	Index    uint64
	NodeA    uint64
	NodeB    uint64
	Deadline uint64
	Winner   uint64
	Evidence common.Hash
}

// GameAction represents an action to take in the game
type GameAction struct {
	Type      ActionType
	Claim     common.Hash
	ParentIdx uint64
	MatchIdx  uint64
	Evidence  []byte
}

// ActionType represents the type of action to take
type ActionType int

const (
	ActionTypeNone ActionType = iota
	ActionTypeJoinTournament
	ActionTypeSubmitEvidence
	ActionTypeResolveMatch
)

// TraceProvider is an interface for generating execution traces
type TraceProvider interface {
	// GetTrace generates an execution trace for a given L2 block number
	GetTrace(ctx context.Context, blockNumber *big.Int) (Trace, error)

	// VerifyTrace verifies that a trace is valid
	VerifyTrace(ctx context.Context, trace Trace) (bool, error)

	// Type returns the type of the trace provider
	Type() string
}

// Trace represents an execution trace
type Trace interface {
	// GenerateClaim generates a claim for the trace
	GenerateClaim() (common.Hash, error)

	// GenerateEvidence generates evidence for a dispute between two claims
	GenerateEvidence(claimA, claimB common.Hash) ([]byte, error)

	// GetStateAtIndex gets the state at a specific index in the trace
	GetStateAtIndex(index uint64) ([]byte, error)

	// GetTraceLength gets the length of the trace
	GetTraceLength() (uint64, error)
}

// Game represents a dispute game
type Game interface {
	// Addr returns the address of the game
	Addr() common.Address

	// Status returns the status of the game
	Status() (GameStatus, error)

	// L2BlockNumber returns the L2 block number associated with this dispute
	L2BlockNumber() (*big.Int, error)
}

// GameStatus represents the status of a game
type GameStatus int

const (
	GameStatusInProgress GameStatus = iota
	GameStatusChallengerWins
	GameStatusDefenderWins
	GameStatusDraw
	GameStatusCancelled
)
