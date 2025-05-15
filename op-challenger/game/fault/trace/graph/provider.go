// Copyright 2023 The Ethereum Authors
// This file is part of the Optimism library.
//
// The Optimism library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Optimism library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.

package graph

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// GraphTraceProvider is a TraceProvider implementation that uses a ComputationGraph
// to provide trace data. It wraps an existing TraceProvider and builds a computation
// graph from it.
type GraphTraceProvider struct {
	logger     log.Logger
	graph      ComputationGraph
	builder    GraphBuilder
	provider   types.TraceProvider
	gameDepth  types.Depth
	buildOnGet bool
}

// NewGraphTraceProvider creates a new GraphTraceProvider
func NewGraphTraceProvider(
	logger log.Logger,
	builder GraphBuilder,
	provider types.TraceProvider,
	gameDepth types.Depth,
	buildOnGet bool,
) *GraphTraceProvider {
	return &GraphTraceProvider{
		logger:     logger,
		builder:    builder,
		provider:   provider,
		gameDepth:  gameDepth,
		buildOnGet: buildOnGet,
	}
}

// ensureGraph ensures that the computation graph is built
func (p *GraphTraceProvider) ensureGraph(ctx context.Context) error {
	if p.graph != nil {
		return nil
	}

	p.logger.Info("Building computation graph")
	graph, err := p.builder.BuildGraph(ctx, p.provider, p.gameDepth)
	if err != nil {
		return fmt.Errorf("failed to build computation graph: %w", err)
	}

	p.graph = graph
	return nil
}

// ensurePartialGraph ensures that a partial computation graph is built around the given position
func (p *GraphTraceProvider) ensurePartialGraph(ctx context.Context, pos types.Position) error {
	// If we already have a full graph, no need to build a partial one
	if p.graph != nil {
		return nil
	}

	p.logger.Info("Building partial computation graph", "position", pos)
	// Build a partial graph around the position with a reasonable depth
	// This allows us to get the node and its immediate context without building the entire graph
	graph, err := p.builder.BuildPartialGraph(ctx, p.provider, pos, 2, p.gameDepth)
	if err != nil {
		return fmt.Errorf("failed to build partial computation graph: %w", err)
	}

	p.graph = graph
	return nil
}

// AbsolutePreStateCommitment implements the PrestateProvider interface
func (p *GraphTraceProvider) AbsolutePreStateCommitment(ctx context.Context) (common.Hash, error) {
	return p.provider.AbsolutePreStateCommitment(ctx)
}

// Get implements the TraceProvider interface
func (p *GraphTraceProvider) Get(ctx context.Context, pos types.Position) (common.Hash, error) {
	if p.buildOnGet {
		if err := p.ensurePartialGraph(ctx, pos); err != nil {
			// Fall back to the underlying provider if we can't build the graph
			p.logger.Warn("Failed to build partial graph, falling back to underlying provider", "err", err)
			return p.provider.Get(ctx, pos)
		}

		node, err := p.graph.GetNodeByPosition(ctx, pos)
		if err != nil {
			if errors.Is(err, ErrNodeNotFound) {
				// Fall back to the underlying provider if the node is not in the graph
				p.logger.Warn("Node not found in graph, falling back to underlying provider", "position", pos)
				return p.provider.Get(ctx, pos)
			}
			return common.Hash{}, err
		}

		return node.Value, nil
	}

	// If we're not building on get, just use the underlying provider
	return p.provider.Get(ctx, pos)
}

// GetStepData implements the TraceProvider interface
func (p *GraphTraceProvider) GetStepData(ctx context.Context, pos types.Position) ([]byte, []byte, *types.PreimageOracleData, error) {
	if p.buildOnGet {
		if err := p.ensurePartialGraph(ctx, pos); err != nil {
			// Fall back to the underlying provider if we can't build the graph
			p.logger.Warn("Failed to build partial graph, falling back to underlying provider", "err", err)
			return p.provider.GetStepData(ctx, pos)
		}

		// Get the current node and its parent
		node, err := p.graph.GetNodeByPosition(ctx, pos)
		if err != nil {
			if errors.Is(err, ErrNodeNotFound) {
				// Fall back to the underlying provider if the node is not in the graph
				p.logger.Warn("Node not found in graph, falling back to underlying provider", "position", pos)
				return p.provider.GetStepData(ctx, pos)
			}
			return nil, nil, nil, err
		}

		// For the root position, there's no parent, so we need to handle it specially
		if pos.IsRootPosition() {
			// For the root position, we need to get the absolute prestate
			prestate := node.Data
			if len(prestate) == 0 {
				return p.provider.GetStepData(ctx, pos)
			}

			// For the root, there's no proof data or preimage data
			return prestate, []byte{}, nil, nil
		}

		// Get the parent position
		parentPos := pos.Parent()
		parentNode, err := p.graph.GetNodeByPosition(ctx, parentPos)
		if err != nil {
			if errors.Is(err, ErrNodeNotFound) {
				// Fall back to the underlying provider if the parent node is not in the graph
				p.logger.Warn("Parent node not found in graph, falling back to underlying provider", "position", parentPos)
				return p.provider.GetStepData(ctx, pos)
			}
			return nil, nil, nil, err
		}

		// Get the step data between the parent and current node
		proofData, preimageData, err := p.graph.GetStepData(ctx, parentNode.ID, node.ID)
		if err != nil {
			// Fall back to the underlying provider if we can't get the step data
			p.logger.Warn("Failed to get step data from graph, falling back to underlying provider", "err", err)
			return p.provider.GetStepData(ctx, pos)
		}

		return parentNode.Data, proofData, preimageData, nil
	}

	// If we're not building on get, just use the underlying provider
	return p.provider.GetStepData(ctx, pos)
}

// GetL2BlockNumberChallenge implements the TraceProvider interface
func (p *GraphTraceProvider) GetL2BlockNumberChallenge(ctx context.Context) (*types.InvalidL2BlockNumberChallenge, error) {
	return p.provider.GetL2BlockNumberChallenge(ctx)
}

// GetGraph returns the underlying computation graph
func (p *GraphTraceProvider) GetGraph(ctx context.Context) (ComputationGraph, error) {
	if err := p.ensureGraph(ctx); err != nil {
		return nil, err
	}
	return p.graph, nil
}

var _ types.TraceProvider = (*GraphTraceProvider)(nil)
