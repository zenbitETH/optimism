// Package multiphase implements the multi-phase dispute resolution system
package multiphase

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
)

var (
	ErrInvalidPhase        = errors.New("invalid dispute phase")
	ErrDisputeNotFound     = errors.New("dispute not found")
	ErrTransitionFailed    = errors.New("failed to transition phase")
	ErrVerificationFailed  = errors.New("verification failed")
	ErrInsufficientWitness = errors.New("insufficient witness data")
)

// MultiPhaseEngine coordinates the multi-phase dispute resolution process
type MultiPhaseEngine struct {
	client             *ethclient.Client
	multiPhaseResolver *MultiPhaseResolver
	phase1Verifier     *Phase1Verifier
	phase2Verifier     *Phase2Verifier
	lazyLoadingManager *LazyLoadingManager
	metrics            metrics.Metricer
}

// Config for creating a new MultiPhaseEngine
type Config struct {
	Client                 *ethclient.Client
	MultiPhaseResolverAddr common.Address
	Metrics                metrics.Metricer
}

// NewMultiPhaseEngine creates a new instance of MultiPhaseEngine
func NewMultiPhaseEngine(ctx context.Context, cfg *Config) (*MultiPhaseEngine, error) {
	multiPhaseResolver, err := NewMultiPhaseResolver(cfg.Client, cfg.MultiPhaseResolverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create multi-phase resolver: %w", err)
	}

	llm := NewLazyLoadingManager(cfg.Client)

	p1v := NewPhase1Verifier(cfg.Client, llm)
	p2v := NewPhase2Verifier(cfg.Client, llm)

	return &MultiPhaseEngine{
		client:             cfg.Client,
		multiPhaseResolver: multiPhaseResolver,
		phase1Verifier:     p1v,
		phase2Verifier:     p2v,
		lazyLoadingManager: llm,
		metrics:            cfg.Metrics,
	}, nil
}

// ProcessDispute handles the dispute resolution process
func (e *MultiPhaseEngine) ProcessDispute(ctx context.Context, disputeID *big.Int) error {
	// Get current phase
	phase, err := e.multiPhaseResolver.GetCurrentPhase(&bind.CallOpts{Context: ctx}, disputeID)
	if err != nil {
		return fmt.Errorf("failed to get current phase: %w", err)
	}

	e.metrics.RecordDisputePhase(disputeID.Uint64(), uint8(phase))

	// Process based on current phase
	switch phase {
	case 1: // Phase 1
		return e.processPhase1(ctx, disputeID)
	case 2: // Phase 2
		return e.processPhase2(ctx, disputeID)
	default:
		return ErrInvalidPhase
	}
}

// processPhase1 handles Phase 1 verification
func (e *MultiPhaseEngine) processPhase1(ctx context.Context, disputeID *big.Int) error {
	// Get dispute details
	details, err := e.getDisputeDetails(ctx, disputeID)
	if err != nil {
		return err
	}

	// Verify state segments and identify discrepancies
	segmentStart, segmentEnd, segmentRoot, err := e.phase1Verifier.VerifyAndFindDiscrepancy(
		ctx,
		disputeID,
		details.P1StartIndex,
		details.P1EndIndex,
		details.P1StartStateRoot,
		details.P1EndStateRoot,
	)
	if err != nil {
		return fmt.Errorf("failed to verify and find discrepancy: %w", err)
	}

	// Record metrics for the verified segment
	segmentSize := new(big.Int).Sub(segmentEnd, segmentStart)
	e.metrics.RecordVerifiedSegment(disputeID.Uint64(), segmentSize.Uint64())

	// If we found a discrepancy in a small enough segment, transition to Phase 2
	threshold := big.NewInt(1000) // This should match the contract's threshold
	if segmentSize.Cmp(threshold) <= 0 {
		auth, err := e.getTransactionOpts(ctx)
		if err != nil {
			return err
		}

		tx, err := e.multiPhaseResolver.TransitionToPhase2(
			auth,
			disputeID,
			segmentStart,
			segmentEnd,
			segmentRoot,
		)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrTransitionFailed, err)
		}

		e.metrics.RecordPhaseTransition(disputeID.Uint64(), 1, 2)

		// Wait for transaction to be mined
		receipt, err := bind.WaitMined(ctx, e.client, tx)
		if err != nil {
			return fmt.Errorf("failed to wait for phase transition: %w", err)
		}

		if receipt.Status != 1 {
			return ErrTransitionFailed
		}
	}

	return nil
}

// processPhase2 handles Phase 2 verification
func (e *MultiPhaseEngine) processPhase2(ctx context.Context, disputeID *big.Int) error {
	// Get dispute details
	details, err := e.getDisputeDetails(ctx, disputeID)
	if err != nil {
		return err
	}

	// Verify individual instructions within the segment
	instructionIndex, witnessData, err := e.phase2Verifier.VerifyAndFindFault(
		ctx,
		disputeID,
		details.P2SegmentStart,
		details.P2SegmentEnd,
		details.P2SegmentStateRoot,
	)
	if err != nil {
		return fmt.Errorf("failed to verify and find fault: %w", err)
	}

	// Submit the witness data for the disputed instruction
	auth, err := e.getTransactionOpts(ctx)
	if err != nil {
		return err
	}

	tx, err := e.multiPhaseResolver.ExecuteDisputedInstruction(
		auth,
		disputeID,
		instructionIndex,
		witnessData,
	)
	if err != nil {
		return fmt.Errorf("failed to execute disputed instruction: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return fmt.Errorf("failed to wait for instruction execution: %w", err)
	}

	if receipt.Status != 1 {
		return ErrVerificationFailed
	}

	return nil
}

// Helper functions

func (e *MultiPhaseEngine) getDisputeDetails(ctx context.Context, disputeID *big.Int) (*DisputeDetails, error) {
	details, err := e.multiPhaseResolver.GetDisputeDetails(&bind.CallOpts{Context: ctx}, disputeID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDisputeNotFound, err)
	}

	// Convert to a more usable format
	return &DisputeDetails{
		Proposer:            details.Proposer,
		Challenger:          details.Challenger,
		CurrentPhase:        uint8(details.CurrentPhase),
		Status:              uint8(details.Status),
		ClaimedStateRoot:    details.ClaimedStateRoot,
		ChallengedStateRoot: details.ChallengedStateRoot,
		Timestamp:           details.Timestamp,
		P1StartIndex:        details.P1StartIndex,
		P1EndIndex:          details.P1EndIndex,
		P1StartStateRoot:    details.P1StartStateRoot,
		P1EndStateRoot:      details.P1EndStateRoot,
		P1BisectionCount:    details.P1BisectionCount,
		P2SegmentStart:      details.P2SegmentStart,
		P2SegmentEnd:        details.P2SegmentEnd,
		P2SegmentStateRoot:  details.P2SegmentStateRoot,
		P2InstructionIndex:  details.P2InstructionIndex,
		P2PreStateRoot:      details.P2PreStateRoot,
		P2PostStateRoot:     details.P2PostStateRoot,
		P2WitnessHash:       details.P2WitnessHash,
	}, nil
}

func (e *MultiPhaseEngine) getTransactionOpts(ctx context.Context) (*bind.TransactOpts, error) {
	// This would be implemented based on the authentication method used
	// For example, using a private key or hardware wallet
	// For this example, we'll leave it as a placeholder
	return nil, nil
}

// DisputeDetails represents the details of a dispute
type DisputeDetails struct {
	Proposer            common.Address
	Challenger          common.Address
	CurrentPhase        uint8
	Status              uint8
	ClaimedStateRoot    [32]byte
	ChallengedStateRoot [32]byte
	Timestamp           *big.Int
	P1StartIndex        *big.Int
	P1EndIndex          *big.Int
	P1StartStateRoot    [32]byte
	P1EndStateRoot      [32]byte
	P1BisectionCount    *big.Int
	P2SegmentStart      *big.Int
	P2SegmentEnd        *big.Int
	P2SegmentStateRoot  [32]byte
	P2InstructionIndex  *big.Int
	P2PreStateRoot      [32]byte
	P2PostStateRoot     [32]byte
	P2WitnessHash       [32]byte
}
