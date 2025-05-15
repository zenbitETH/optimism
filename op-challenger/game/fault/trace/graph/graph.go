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
	"fmt"
	"sync"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// OpMLGraph extends the inMemoryGraph to support opML architectural principles.
// It adds support for operations, ports, and tensors.
type OpMLGraph struct {
	*inMemoryGraph
	operations map[string]Operation
	logger     log.Logger
	mu         sync.RWMutex
}

// NewOpMLGraph creates a new OpMLGraph with the given logger.
func NewOpMLGraph(logger log.Logger) *OpMLGraph {
	return &OpMLGraph{
		inMemoryGraph: newInMemoryGraph(),
		operations:    make(map[string]Operation),
		logger:        logger,
	}
}

// AddOperation implements the ComputationGraph interface.
func (g *OpMLGraph) AddOperation(ctx context.Context, op Operation) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if the operation already exists
	if _, exists := g.operations[op.ID()]; exists {
		return fmt.Errorf("%w: operation %s already exists", ErrInvalidOperation, op.ID())
	}

	g.operations[op.ID()] = op
	return nil
}

// GetOperations implements the ComputationGraph interface.
func (g *OpMLGraph) GetOperations(ctx context.Context) ([]Operation, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Return a copy of the operations
	operations := make([]Operation, 0, len(g.operations))
	for _, op := range g.operations {
		operations = append(operations, op)
	}

	return operations, nil
}

// GetOperationByID implements the ComputationGraph interface.
func (g *OpMLGraph) GetOperationByID(ctx context.Context, id string) (Operation, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	op, exists := g.operations[id]
	if !exists {
		return nil, fmt.Errorf("%w: operation %s not found", ErrInvalidOperation, id)
	}

	return op, nil
}

// OpMLGraphBuilder extends the BaseGraphBuilder to support opML architectural principles.
// It builds a computation graph with operations, ports, and tensors.
type OpMLGraphBuilder struct {
	*BaseGraphBuilder
}

// NewOpMLGraphBuilder creates a new OpMLGraphBuilder with the given logger.
func NewOpMLGraphBuilder(logger log.Logger) *OpMLGraphBuilder {
	return &OpMLGraphBuilder{
		BaseGraphBuilder: NewBaseGraphBuilder(logger),
	}
}

// BuildGraph implements the GraphBuilder interface.
func (b *OpMLGraphBuilder) BuildGraph(ctx context.Context, traceProvider types.TraceProvider, gameDepth types.Depth) (ComputationGraph, error) {
	// Create a new OpMLGraph
	graph := NewOpMLGraph(b.logger)

	// Build the base graph
	baseGraph, err := BuildTraceGraph(
		ctx,
		b.logger,
		traceProvider,
		gameDepth,
		b.buildNode,
		b.buildEdge,
	)
	if err != nil {
		return nil, err
	}

	// Copy the nodes and edges from the base graph to the OpMLGraph
	// This is a simplified approach; in a real implementation, we would
	// build the operations, ports, and tensors as we build the graph

	// Get the root node
	rootNode, err := baseGraph.GetRootNode(ctx)
	if err != nil {
		return nil, err
	}

	// Add the root node to the graph
	graph.AddNode(rootNode)

	// Build the operations for the graph
	if err := b.buildOperations(ctx, graph, baseGraph, gameDepth); err != nil {
		return nil, err
	}

	return graph, nil
}

// BuildPartialGraph implements the GraphBuilder interface.
func (b *OpMLGraphBuilder) BuildPartialGraph(ctx context.Context, traceProvider types.TraceProvider, pos types.Position, depth uint64, gameDepth types.Depth) (ComputationGraph, error) {
	// Create a new OpMLGraph
	graph := NewOpMLGraph(b.logger)

	// Build the base graph
	baseGraph, err := BuildPartialTraceGraph(
		ctx,
		b.logger,
		traceProvider,
		pos,
		depth,
		gameDepth,
		b.buildNode,
		b.buildEdge,
	)
	if err != nil {
		return nil, err
	}

	// Copy the nodes and edges from the base graph to the OpMLGraph
	// This is a simplified approach; in a real implementation, we would
	// build the operations, ports, and tensors as we build the graph

	// Get the node at the specified position
	node, err := baseGraph.GetNodeByPosition(ctx, pos)
	if err != nil {
		return nil, err
	}

	// Add the node to the graph
	graph.AddNodeWithPosition(node, pos)

	// Build the operations for the graph
	if err := b.buildOperations(ctx, graph, baseGraph, gameDepth); err != nil {
		return nil, err
	}

	return graph, nil
}

// buildNode creates a new Node for the graph.
func (b *OpMLGraphBuilder) buildNode(pos types.Position, value common.Hash, data []byte) (*Node, error) {
	// Create a unique ID for the node
	id := NodeID(fmt.Sprintf("node-%s", pos))

	// Create the node
	node := &Node{
		ID:       id,
		Value:    value,
		Data:     data,
		Parents:  make([]NodeID, 0),
		Children: make([]NodeID, 0),
		Metadata: make(map[string]interface{}),
	}

	// Add position information to the metadata
	node.Metadata["position"] = pos.String()

	return node, nil
}

// buildEdge creates a new Edge between two nodes.
func (b *OpMLGraphBuilder) buildEdge(fromNode, toNode *Node, fromPos, toPos types.Position) (*Edge, error) {
	// Create the edge
	edge := &Edge{
		From:     fromNode.ID,
		To:       toNode.ID,
		Type:     "transition",
		Metadata: make(map[string]interface{}),
	}

	// Add position information to the metadata
	edge.Metadata["fromPosition"] = fromPos.String()
	edge.Metadata["toPosition"] = toPos.String()

	return edge, nil
}

// buildOperations builds the operations for the graph.
func (b *OpMLGraphBuilder) buildOperations(ctx context.Context, graph *OpMLGraph, baseGraph ComputationGraph, gameDepth types.Depth) error {
	// Get all nodes in the graph
	nodeCount, err := baseGraph.GetNodeCount(ctx)
	if err != nil {
		return err
	}

	// For each node, create an operation
	for i := uint64(0); i < nodeCount; i++ {
		// This is a simplified approach; in a real implementation, we would
		// have a more sophisticated way to iterate over the nodes

		// Get the root node
		rootNode, err := baseGraph.GetRootNode(ctx)
		if err != nil {
			return err
		}

		// Create an operation for the root node
		opID := fmt.Sprintf("op-%s", rootNode.ID)
		op := NewBaseOperation(opID, fmt.Sprintf("Operation %s", rootNode.ID), "state_transition")

		// Add input and output ports
		inputPort := NewBasePort(fmt.Sprintf("%s-in", opID), "input", PortDirectionInput, op)
		outputPort := NewBasePort(fmt.Sprintf("%s-out", opID), "output", PortDirectionOutput, op)

		op.AddInputPort(inputPort)
		op.AddOutputPort(outputPort)

		// Create tensors for the ports
		inputTensor := NewBaseTensor(fmt.Sprintf("%s-in-tensor", opID), "input_tensor", []int{1}, TensorDataTypeBytes)
		outputTensor := NewBaseTensor(fmt.Sprintf("%s-out-tensor", opID), "output_tensor", []int{1}, TensorDataTypeBytes)

		// Set the data for the tensors
		if err := inputTensor.SetData(rootNode.Data); err != nil {
			return err
		}

		// For the output tensor, we use the node's value
		if err := outputTensor.SetData(rootNode.Value.Bytes()); err != nil {
			return err
		}

		// Set the tensors for the ports
		if err := inputPort.SetTensor(inputTensor); err != nil {
			return err
		}

		if err := outputPort.SetTensor(outputTensor); err != nil {
			return err
		}

		// Add the operation to the graph
		if err := graph.AddOperation(ctx, op); err != nil {
			return err
		}

		// Get the children of the node
		children, err := baseGraph.GetChildren(ctx, rootNode.ID)
		if err != nil {
			return err
		}

		// For each child, create an operation and connect it to the parent
		for _, child := range children {
			childOpID := fmt.Sprintf("op-%s", child.ID)
			childOp := NewBaseOperation(childOpID, fmt.Sprintf("Operation %s", child.ID), "state_transition")

			// Add input and output ports
			childInputPort := NewBasePort(fmt.Sprintf("%s-in", childOpID), "input", PortDirectionInput, childOp)
			childOutputPort := NewBasePort(fmt.Sprintf("%s-out", childOpID), "output", PortDirectionOutput, childOp)

			childOp.AddInputPort(childInputPort)
			childOp.AddOutputPort(childOutputPort)

			// Create tensors for the ports
			childInputTensor := NewBaseTensor(fmt.Sprintf("%s-in-tensor", childOpID), "input_tensor", []int{1}, TensorDataTypeBytes)
			childOutputTensor := NewBaseTensor(fmt.Sprintf("%s-out-tensor", childOpID), "output_tensor", []int{1}, TensorDataTypeBytes)

			// Set the data for the tensors
			if err := childInputTensor.SetData(child.Data); err != nil {
				return err
			}

			// For the output tensor, we use the node's value
			if err := childOutputTensor.SetData(child.Value.Bytes()); err != nil {
				return err
			}

			// Set the tensors for the ports
			if err := childInputPort.SetTensor(childInputTensor); err != nil {
				return err
			}

			if err := childOutputPort.SetTensor(childOutputTensor); err != nil {
				return err
			}

			// Add the operation to the graph
			if err := graph.AddOperation(ctx, childOp); err != nil {
				return err
			}

			// Connect the parent's output port to the child's input port
			if err := outputPort.Connect(childInputPort); err != nil {
				return err
			}
		}
	}

	return nil
}

// Ensure OpMLGraph implements ComputationGraph
var _ ComputationGraph = (*OpMLGraph)(nil)

// Ensure OpMLGraphBuilder implements GraphBuilder
var _ GraphBuilder = (*OpMLGraphBuilder)(nil)
