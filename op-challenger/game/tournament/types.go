// Package tournament provides functionality for interacting with tournament-based dispute games
// in the Cartesi DAVE fraud proofs system.
package tournament

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum/go-ethereum/common"
)

// TraceProvider is an interface for accessing execution traces and generating/verifying proofs.
// It abstracts the underlying trace generation mechanism, supporting both 64-bit MIPS (Cannon)
// and RISC-V (Cartesi) architectures.
type TraceProvider interface {
	// GetTrace returns the trace at the specified index
	GetTrace(ctx context.Context, idx uint64) (common.Hash, error)

	// GenerateProof generates a proof for the specified trace index
	GenerateProof(ctx context.Context, idx uint64) ([]byte, error)

	// VerifyProof verifies a proof against a claim
	VerifyProof(ctx context.Context, claim common.Hash, proof []byte) (bool, error)

	// GetArchitecture returns the architecture type used by this trace provider
	GetArchitecture() string
}

// TournamentContract defines the interface for interacting with tournament contracts
type TournamentContract interface {
	// GetStatus returns the current status of the tournament
	GetStatus(ctx context.Context) (types.GameStatus, error)

	// GetTotalParticipants returns the total number of participants in the tournament
	GetTotalParticipants(ctx context.Context) (uint64, error)

	// GetCurrentRound returns the current round of the tournament
	GetCurrentRound(ctx context.Context) (uint64, error)

	// GetMatchCount returns the total number of matches in the tournament
	GetMatchCount(ctx context.Context) (uint64, error)

	// HasParticipated checks if an address has participated in the tournament
	HasParticipated(ctx context.Context, addr common.Address) (bool, error)

	// JoinTournament joins the tournament with a counter-claim
	JoinTournament(ctx context.Context, claim common.Hash) error

	// ResolveMatch resolves a match in the tournament
	ResolveMatch(ctx context.Context, matchIndex uint64, winnerIndex uint64) error

	// ClaimBond claims the bond as the tournament winner
	ClaimBond(ctx context.Context) error

	// GetL1Head returns the L1 head hash at the time the tournament was created
	GetL1Head(ctx context.Context) (common.Hash, error)

	// GetRootClaim returns the root claim of the tournament
	GetRootClaim(ctx context.Context) (common.Hash, error)

	// GetMatch returns information about a match in the tournament
	GetMatch(ctx context.Context, matchIndex uint64) (Match, error)

	// GetNode returns information about a node in the tournament tree
	GetNode(ctx context.Context, nodeIndex uint64) (Node, error)
}

// TxSender defines the interface for sending transactions
type TxSender interface {
	// SendAndWaitSimple sends a transaction and waits for it to be mined
	SendAndWaitSimple(name string, tx TxCandidate) error

	// From returns the sender address
	From() common.Address
}

// TxCandidate represents a transaction to be sent
// This matches the current definition in the OP Stack
type TxCandidate struct {
	// TxData is the transaction calldata to be used in the constructed tx.
	TxData []byte
	// To is the recipient of the constructed tx. Nil means contract creation.
	To *common.Address
	// GasLimit is the gas limit to be used in the constructed tx.
	GasLimit uint64
	// Value is the value to be used in the constructed tx.
	Value *big.Int
}

// Match represents a match in the tournament
type Match struct {
	NodeA     uint64
	NodeB     uint64
	Winner    uint64
	StartTime uint64
	Resolved  bool
}

// Node represents a node in the tournament tree
type Node struct {
	Participant common.Address
	Claim       common.Hash
	HasJoined   bool
	BondAmount  uint64
}

// TournamentMetrics tracks metrics for tournament operations
type TournamentMetrics struct {
	TotalMatches      uint64
	ResolvedMatches   uint64
	ActiveMatches     uint64
	TotalParticipants uint64
	CurrentRound      uint64
	LastUpdateTime    time.Time
}

// RISCVMachineConfig contains configuration parameters for the RISC-V machine emulator.
type RISCVMachineConfig struct {
	// RomFilePath is the path to the ROM file for the RISC-V machine
	RomFilePath string
	// RamSize is the size of RAM in bytes for the RISC-V machine
	RamSize uint64
	// KernelFilePath is the path to the Linux kernel image for the RISC-V machine
	KernelFilePath string
	// RootFSPath is the path to the root filesystem for the RISC-V machine
	RootFSPath string
	// MaxCycles is the maximum number of cycles to execute
	MaxCycles uint64
}
