// Package tournament provides functionality for interacting with tournament-based dispute games
// in the Cartesi DAVE fraud proofs system.
package tournament

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// TournamentTraceProvider provides traces for tournament games.
// It implements the TraceProvider interface and supports caching to improve performance.
type TournamentTraceProvider struct {
	logger        log.Logger
	traceAccessor types.TraceAccessor
	proofCache    map[uint64][]byte
	cacheMu       sync.RWMutex
	architecture  string
}

// NewTournamentTraceProvider creates a new TournamentTraceProvider with the specified
// trace accessor and architecture. If no architecture is specified, it defaults to
// 64-bit MIPS (Cannon).
//
// Parameters:
//   - logger: Logger for recording trace provider operations
//   - traceAccessor: Interface for accessing execution traces
//   - architecture: Architecture type ("mips64" or "riscv")
//
// Returns:
//   - A new TournamentTraceProvider instance
func NewTournamentTraceProvider(
	logger log.Logger,
	traceAccessor types.TraceAccessor,
	architecture string,
) *TournamentTraceProvider {
	if architecture == "" {
		// Default to 64-bit MIPS (Cannon) if not specified
		architecture = "mips64"
	}

	return &TournamentTraceProvider{
		logger:        logger.New("component", "TournamentTraceProvider", "arch", architecture),
		traceAccessor: traceAccessor,
		proofCache:    make(map[uint64][]byte),
		architecture:  architecture,
	}
}

// GetTrace returns the trace at the specified index.
// It retrieves the trace from the underlying trace accessor and returns it as a hash.
//
// Parameters:
//   - ctx: Context for the operation
//   - idx: Index of the trace to retrieve
//
// Returns:
//   - The trace hash at the specified index
//   - Error if the trace could not be retrieved
func (t *TournamentTraceProvider) GetTrace(ctx context.Context, game types.Game, ref types.Claim, pos types.Position, idx uint64) (common.Hash, error) {
	t.logger.Debug("Getting trace", "index", idx)
	value, err := t.traceAccessor.Get(ctx, game, ref, pos)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get trace at index %d: %w", idx, err)
	}
	return value, nil
}

// GenerateProof generates a proof for the specified trace index.
// It uses the appropriate proof generation mechanism based on the configured architecture.
// Proofs are cached to avoid redundant computation.
//
// Parameters:
//   - ctx: Context for the operation
//   - idx: Index of the trace to generate a proof for
//
// Returns:
//   - The generated proof as a byte array
//   - Error if the proof could not be generated
func (t *TournamentTraceProvider) GenerateProof(ctx context.Context, game types.Game, ref types.Claim, pos types.Position, idx uint64) ([]byte, error) {
	t.logger.Debug("Generating proof", "index", idx, "architecture", t.architecture)

	// Check cache first
	t.cacheMu.RLock()
	cachedProof, exists := t.proofCache[idx]
	t.cacheMu.RUnlock()

	if exists {
		t.logger.Debug("Using cached proof", "index", idx)
		return cachedProof, nil
	}

	// Generate proof based on architecture
	var proof []byte
	var err error

	switch t.architecture {
	case "mips64":
		proof, err = t.generateMips64Proof(ctx, game, ref, pos, idx)
	case "riscv":
		proof, err = t.generateRiscVProof(ctx, game, ref, pos, idx)
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", t.architecture)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate proof for index %d: %w", idx, err)
	}

	// Cache the proof
	t.cacheMu.Lock()
	t.proofCache[idx] = proof
	t.cacheMu.Unlock()

	return proof, nil
}

// generateMips64Proof generates a proof using the 64-bit MIPS architecture (Cannon).
// This method is used internally by GenerateProof when the architecture is set to "mips64".
//
// Parameters:
//   - ctx: Context for the operation
//   - idx: Index of the trace to generate a proof for
//
// Returns:
//   - The generated MIPS64 proof as a byte array
//   - Error if the proof could not be generated
func (t *TournamentTraceProvider) generateMips64Proof(ctx context.Context, game types.Game, ref types.Claim, pos types.Position, idx uint64) ([]byte, error) {
	// Get the trace value
	value, err := t.traceAccessor.Get(ctx, game, ref, pos)
	if err != nil {
		return nil, fmt.Errorf("failed to get trace for MIPS64 proof at index %d: %w", idx, err)
	}

	// In a production implementation, this would use the Cannon prover to generate
	// a proper MIPS64 execution proof. For now, we're creating a structured proof
	// that includes the architecture identifier and trace value.
	proof := append([]byte("mips64:"), value.Bytes()...)

	t.logger.Debug("Generated MIPS64 proof", "index", idx, "proof_size", len(proof))
	return proof, nil
}

// generateRiscVProof generates a proof using the RISC-V architecture (Cartesi).
// This method is used internally by GenerateProof when the architecture is set to "riscv".
//
// Parameters:
//   - ctx: Context for the operation
//   - idx: Index of the trace to generate a proof for
//
// Returns:
//   - The generated RISC-V proof as a byte array
//   - Error if the proof could not be generated
func (t *TournamentTraceProvider) generateRiscVProof(ctx context.Context, game types.Game, ref types.Claim, pos types.Position, idx uint64) ([]byte, error) {
	// Get the trace value
	value, err := t.traceAccessor.Get(ctx, game, ref, pos)
	if err != nil {
		return nil, fmt.Errorf("failed to get trace for RISC-V proof at index %d: %w", idx, err)
	}

	// In a production implementation, this would use the Cartesi Machine to generate
	// a proper RISC-V execution proof. For now, we're creating a structured proof
	// that includes the architecture identifier and trace value.
	proof := append([]byte("riscv:"), value.Bytes()...)

	t.logger.Debug("Generated RISC-V proof", "index", idx, "proof_size", len(proof))
	return proof, nil
}

// VerifyProof verifies a proof against a claim.
// It determines the architecture from the proof prefix and uses the appropriate
// verification mechanism.
//
// Parameters:
//   - ctx: Context for the operation
//   - claim: The claim to verify against
//   - proof: The proof to verify
//
// Returns:
//   - True if the proof is valid for the claim, false otherwise
//   - Error if the verification process failed
func (t *TournamentTraceProvider) VerifyProof(ctx context.Context, claim common.Hash, proof []byte) (bool, error) {
	if len(proof) < 7 {
		return false, fmt.Errorf("proof too short: %d bytes", len(proof))
	}

	t.logger.Debug("Verifying proof", "claim", claim.Hex(), "proof_size", len(proof))

	// Determine the architecture from the proof prefix
	var proofArch string
	if len(proof) >= 7 && string(proof[:6]) == "mips64" {
		proofArch = "mips64"
		proof = proof[7:] // Remove prefix
	} else if len(proof) >= 6 && string(proof[:5]) == "riscv" {
		proofArch = "riscv"
		proof = proof[6:] // Remove prefix
	} else {
		return false, fmt.Errorf("unknown proof architecture")
	}

	// Verify based on architecture
	switch proofArch {
	case "mips64":
		return t.verifyMips64Proof(ctx, claim, proof)
	case "riscv":
		return t.verifyRiscVProof(ctx, claim, proof)
	default:
		return false, fmt.Errorf("unsupported proof architecture: %s", proofArch)
	}
}

// verifyMips64Proof verifies a MIPS64 proof against a claim.
// This method is used internally by VerifyProof when the proof architecture is "mips64".
//
// Parameters:
//   - ctx: Context for the operation
//   - claim: The claim to verify against
//   - proof: The MIPS64 proof to verify (without the prefix)
//
// Returns:
//   - True if the proof is valid for the claim, false otherwise
//   - Error if the verification process failed
func (t *TournamentTraceProvider) verifyMips64Proof(ctx context.Context, claim common.Hash, proof []byte) (bool, error) {
	// In a production implementation, this would use the Cannon verifier to verify
	// the MIPS64 execution proof. For now, we're doing a simple comparison.
	proofHash := common.BytesToHash(proof)

	t.logger.Debug("Verifying MIPS64 proof", "claim", claim.Hex(), "proof_hash", proofHash.Hex())

	// Simple verification for demonstration
	return claim == proofHash, nil
}

// verifyRiscVProof verifies a RISC-V proof against a claim.
// This method is used internally by VerifyProof when the proof architecture is "riscv".
//
// Parameters:
//   - ctx: Context for the operation
//   - claim: The claim to verify against
//   - proof: The RISC-V proof to verify (without the prefix)
//
// Returns:
//   - True if the proof is valid for the claim, false otherwise
//   - Error if the verification process failed
func (t *TournamentTraceProvider) verifyRiscVProof(ctx context.Context, claim common.Hash, proof []byte) (bool, error) {
	// In a production implementation, this would use the Cartesi Machine to verify
	// the RISC-V execution proof. For now, we're doing a simple comparison.
	proofHash := common.BytesToHash(proof)

	t.logger.Debug("Verifying RISC-V proof", "claim", claim.Hex(), "proof_hash", proofHash.Hex())

	// Simple verification for demonstration
	return claim == proofHash, nil
}

// GetArchitecture returns the architecture type used by this trace provider.
// This can be used to determine which proof generation and verification mechanisms
// are being used.
//
// Returns:
//   - The architecture type ("mips64" or "riscv")
func (t *TournamentTraceProvider) GetArchitecture() string {
	return t.architecture
}
