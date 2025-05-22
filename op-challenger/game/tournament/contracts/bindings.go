// SPDX-License-Identifier: MIT
package contracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-service/sources/eth"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

// Bindings contains the generated bindings for the Tournament contracts.
type Bindings struct {
	Tournament         *bindings.TournamentContract
	TournamentFactory  *bindings.TournamentFactoryContract
	DisputeGameFactory *bindings.DisputeGameFactoryContract
}

// Node represents a node in the tournament tree from the contract.
type Node struct {
	Claim      common.Hash
	Claimant   common.Address
	Timestamp  uint64
	Challenged bool
}

// Match represents a match in the tournament from the contract.
type Match struct {
	NodeIndex1 uint64
	NodeIndex2 uint64
	Deadline   uint64
	Winner     uint64
	Evidence   common.Hash
}

// GenerateBindings generates the contract bindings from the network.
func GenerateBindings(
	client *ethclient.Client,
	tournamentAddr common.Address,
	tournamentFactoryAddr common.Address,
	disputeGameFactoryAddr common.Address,
) (*Bindings, error) {
	tournament, err := bindings.NewTournamentContract(tournamentAddr, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create tournament binding: %w", err)
	}

	tournamentFactory, err := bindings.NewTournamentFactoryContract(tournamentFactoryAddr, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create tournament factory binding: %w", err)
	}

	disputeGameFactory, err := bindings.NewDisputeGameFactoryContract(disputeGameFactoryAddr, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create dispute game factory binding: %w", err)
	}

	return &Bindings{
		Tournament:         tournament,
		TournamentFactory:  tournamentFactory,
		DisputeGameFactory: disputeGameFactory,
	}, nil
}

// Tournament represents a binding to the Tournament contract.
type Tournament struct {
	address  common.Address
	contract *bindings.TournamentContract
	client   eth.Client
	logger   log.Logger
}

// NewTournament creates a new Tournament binding.
func NewTournament(address common.Address, client eth.Client, logger log.Logger) (*Tournament, error) {
	contract, err := bindings.NewTournamentContract(address, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind to tournament contract: %w", err)
	}

	return &Tournament{
		address:  address,
		contract: contract,
		client:   client,
		logger:   logger,
	}, nil
}

// GetNodeCount retrieves the number of nodes in the tournament.
func (t *Tournament) GetNodeCount(ctx context.Context) (uint64, error) {
	opts := &bind.CallOpts{Context: ctx}
	count, err := t.contract.GetNodeCount(opts)
	if err != nil {
		return 0, fmt.Errorf("failed to get node count: %w", err)
	}
	return count.Uint64(), nil
}

// GetNode retrieves a node by index from the tournament.
func (t *Tournament) GetNode(ctx context.Context, index uint64) (Node, error) {
	opts := &bind.CallOpts{Context: ctx}
	node, err := t.contract.GetNode(opts, new(big.Int).SetUint64(index))
	if err != nil {
		return Node{}, fmt.Errorf("failed to get node %d: %w", index, err)
	}

	return Node{
		Claim:      node.Claim,
		Claimant:   node.Claimant,
		Timestamp:  node.Timestamp.Uint64(),
		Challenged: node.Challenged,
	}, nil
}

// GetMatchCount retrieves the number of matches in the tournament.
func (t *Tournament) GetMatchCount(ctx context.Context) (uint64, error) {
	opts := &bind.CallOpts{Context: ctx}
	count, err := t.contract.GetMatchCount(opts)
	if err != nil {
		return 0, fmt.Errorf("failed to get match count: %w", err)
	}
	return count.Uint64(), nil
}

// GetMatch retrieves a match by index from the tournament.
func (t *Tournament) GetMatch(ctx context.Context, index uint64) (Match, error) {
	opts := &bind.CallOpts{Context: ctx}
	match, err := t.contract.GetMatch(opts, new(big.Int).SetUint64(index))
	if err != nil {
		return Match{}, fmt.Errorf("failed to get match %d: %w", index, err)
	}

	return Match{
		NodeIndex1: match.NodeIndex1.Uint64(),
		NodeIndex2: match.NodeIndex2.Uint64(),
		Deadline:   match.Deadline.Uint64(),
		Winner:     match.Winner.Uint64(),
		Evidence:   match.Evidence,
	}, nil
}

// Status retrieves the current status of the tournament.
func (t *Tournament) Status(ctx context.Context) (uint8, error) {
	opts := &bind.CallOpts{Context: ctx}
	status, err := t.contract.Status(opts)
	if err != nil {
		return 0, fmt.Errorf("failed to get status: %w", err)
	}
	return status, nil
}

// L2BlockNumber retrieves the L2 block number associated with this tournament.
func (t *Tournament) L2BlockNumber(ctx context.Context) (*big.Int, error) {
	opts := &bind.CallOpts{Context: ctx}
	blockNumber, err := t.contract.L2BlockNumber(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get L2 block number: %w", err)
	}
	return blockNumber, nil
}

// JoinTournament joins the tournament with a counter-claim.
func (t *Tournament) JoinTournament(ctx context.Context, auth *bind.TransactOpts, claim common.Hash, parentClaimIndex uint64) (*common.Hash, error) {
	tx, err := t.contract.JoinTournament(auth, claim, new(big.Int).SetUint64(parentClaimIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to join tournament: %w", err)
	}

	t.logger.Info("Joined tournament", "tx", tx.Hash(), "claim", claim.Hex(), "parentIndex", parentClaimIndex)
	return &tx.Hash(), nil
}

// SubmitEvidence submits evidence for a match.
func (t *Tournament) SubmitEvidence(ctx context.Context, auth *bind.TransactOpts, matchIndex uint64, evidence common.Hash) (*common.Hash, error) {
	tx, err := t.contract.SubmitEvidence(auth, new(big.Int).SetUint64(matchIndex), evidence)
	if err != nil {
		return nil, fmt.Errorf("failed to submit evidence: %w", err)
	}

	t.logger.Info("Submitted evidence", "tx", tx.Hash(), "matchIndex", matchIndex, "evidence", evidence.Hex())
	return &tx.Hash(), nil
}

// ResolveMatch resolves a match in the tournament.
func (t *Tournament) ResolveMatch(ctx context.Context, auth *bind.TransactOpts, matchIndex uint64) (*common.Hash, error) {
	tx, err := t.contract.ResolveMatch(auth, new(big.Int).SetUint64(matchIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve match: %w", err)
	}

	t.logger.Info("Resolved match", "tx", tx.Hash(), "matchIndex", matchIndex)
	return &tx.Hash(), nil
}

// Result retrieves the result of the tournament.
func (t *Tournament) Result(ctx context.Context) (uint8, common.Hash, error) {
	opts := &bind.CallOpts{Context: ctx}
	status, winningClaim, err := t.contract.Result(opts)
	if err != nil {
		return 0, common.Hash{}, fmt.Errorf("failed to get result: %w", err)
	}
	return status, winningClaim, nil
}
