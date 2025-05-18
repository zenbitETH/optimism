package multiphase

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	ErrInstructionVerificationFailed = errors.New("instruction verification failed")
	ErrWitnessGenerationFailed       = errors.New("witness generation failed")
	ErrFaultNotFound                 = errors.New("fault not found in segment")
)

// Phase2Verifier performs fine-grained verification of specific instructions
type Phase2Verifier struct {
	client             *ethclient.Client
	lazyLoadingManager *LazyLoadingManager
}

// NewPhase2Verifier creates a new instance of Phase2Verifier
func NewPhase2Verifier(client *ethclient.Client, llm *LazyLoadingManager) *Phase2Verifier {
	return &Phase2Verifier{
		client:             client,
		lazyLoadingManager: llm,
	}
}

// VerifyAndFindFault identifies the exact instruction causing a fault
func (v *Phase2Verifier) VerifyAndFindFault(
	ctx context.Context,
	disputeID *big.Int,
	segmentStart *big.Int,
	segmentEnd *big.Int,
	segmentStateRoot [32]byte,
) (*big.Int, []byte, error) {
	// Perform binary search on the instructions in the segment
	currentStart := new(big.Int).Set(segmentStart)
	currentEnd := new(big.Int).Set(segmentEnd)

	for currentStart.Cmp(currentEnd) < 0 {
		// If we're down to one instruction, verify it
		if new(big.Int).Sub(currentEnd, currentStart).Cmp(big.NewInt(1)) <= 0 {
			return currentStart, v.generateWitness(ctx, disputeID, currentStart)
		}

		// Find midpoint
		midpoint := new(big.Int).Add(currentStart, currentEnd)
		midpoint.Div(midpoint, big.NewInt(2))

		// Load state at midpoint
		midStateRootBytes, err := v.lazyLoadingManager.LoadState(ctx, disputeID, midpoint)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load state at midpoint: %w", err)
		}

		midStateRoot := common.BytesToHash(midStateRootBytes)

		// Verify instruction execution from start to midpoint
		valid, err := v.VerifyInstructionExecution(
			ctx,
			disputeID,
			currentStart,
			midpoint,
			segmentStateRoot,
			midStateRoot,
		)
		if err != nil {
			return nil, nil, err
		}

		// Determine which half contains the fault
		if !valid {
			// Fault is in the first half
			currentEnd = midpoint
		} else {
			// Fault is in the second half
			currentStart = midpoint
		}
	}

	return nil, nil, ErrFaultNotFound
}

// VerifyInstructionExecution verifies the execution of instructions in a segment
func (v *Phase2Verifier) VerifyInstructionExecution(
	ctx context.Context,
	disputeID *big.Int,
	startIndex *big.Int,
	endIndex *big.Int,
	startStateRoot [32]byte,
	expectedEndStateRoot [32]byte,
) (bool, error) {
	// This would use Cannon FPVM to verify the execution
	// In a real implementation, this would:
	// 1. Use Cannon to execute the instructions in the segment
	// 2. Compare the resulting state root with the expected one

	// For demonstration, we'll simulate verification
	// In production, this would call Cannon or verify against a proven state

	return true, nil
}

// generateWitness generates witness data for a disputed instruction
func (v *Phase2Verifier) generateWitness(
	ctx context.Context,
	disputeID *big.Int,
	instructionIndex *big.Int,
) ([]byte, error) {
	// In a real implementation, this would:
	// 1. Retrieve the instruction details
	// 2. Generate the witness data including memory proofs, register proofs, etc.
	// 3. Format the data according to the expected structure

	// For demonstration, we'll return dummy witness data
	// In production, this would call Cannon to generate proper witness data
	return []byte{}, nil
}
