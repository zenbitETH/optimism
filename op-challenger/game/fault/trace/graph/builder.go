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

// BaseGraphBuilder provides common functionality for building computation graphs
type BaseGraphBuilder struct {
	logger log.Logger
}

// NewBaseGraphBuilder creates a new BaseGraphBuilder
func NewBaseGraphBuilder(logger log.Logger) *BaseGraphBuilder {
	return &BaseGraphBuilder{
		logger: logger,
	}
}

// inMemoryGraph is an in-memory implementation of the ComputationGraph interface
type inMemoryGraph struct {
	nodes      map[NodeID]*Node
	edges      map[string]*Edge
	nodesByPos map[string]*Node
	operations map[string]Operation
	rootNode   *Node
	leafNodes  []*Node
	mu         sync.RWMutex
}

// newInMemoryGraph creates a new empty in-memory graph
func newInMemoryGraph() *inMemoryGraph {
	return &inMemoryGraph{
		nodes:      make(map[NodeID]*Node),
		edges:      make(map[string]*Edge),
		nodesByPos: make(map[string]*Node),
		operations: make(map[string]Operation),
	}
}

// positionKey creates a string key from a position for use in the nodesByPos map
func positionKey(pos types.Position) string {
	return pos.String()
}

// edgeKey creates a string key from two node IDs for use in the edges map
func edgeKey(from, to NodeID) string {
	return fmt.Sprintf("%s->%s", from, to)
}

// AddNode adds a node to the graph
func (g *inMemoryGraph) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node

	// If this is the first node, it's the root
	if len(g.nodes) == 1 {
		g.rootNode = node
	}

	// If the node has no children, it's a leaf
	if len(node.Children) == 0 {
		g.leafNodes = append(g.leafNodes, node)
	} else {
		// If it has children, make sure it's not in the leaf nodes
		for i, leaf := range g.leafNodes {
			if leaf.ID == node.ID {
				// Remove it from leaf nodes
				g.leafNodes = append(g.leafNodes[:i], g.leafNodes[i+1:]...)
				break
			}
		}
	}
}

// AddNodeWithPosition adds a node to the graph and associates it with a position
func (g *inMemoryGraph) AddNodeWithPosition(node *Node, pos types.Position) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node
	g.nodesByPos[positionKey(pos)] = node

	// If this is the first node, it's the root
	if len(g.nodes) == 1 {
		g.rootNode = node
	}

	// If the node has no children, it's a leaf
	if len(node.Children) == 0 {
		g.leafNodes = append(g.leafNodes, node)
	} else {
		// If it has children, make sure it's not in the leaf nodes
		for i, leaf := range g.leafNodes {
			if leaf.ID == node.ID {
				// Remove it from leaf nodes
				g.leafNodes = append(g.leafNodes[:i], g.leafNodes[i+1:]...)
				break
			}
		}
	}
}

// AddEdge adds an edge to the graph
func (g *inMemoryGraph) AddEdge(edge *Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Ensure both nodes exist
	fromNode, exists := g.nodes[edge.From]
	if !exists {
		return fmt.Errorf("%w: from node %s not found", ErrNodeNotFound, edge.From)
	}

	toNode, exists := g.nodes[edge.To]
	if !exists {
		return fmt.Errorf("%w: to node %s not found", ErrNodeNotFound, edge.To)
	}

	// Add the edge
	key := edgeKey(edge.From, edge.To)
	g.edges[key] = edge

	// Update the node relationships
	fromNode.Children = append(fromNode.Children, edge.To)
	toNode.Parents = append(toNode.Parents, edge.From)

	// If the from node was a leaf, it's no longer a leaf
	for i, leaf := range g.leafNodes {
		if leaf.ID == fromNode.ID {
			// Remove it from leaf nodes
			g.leafNodes = append(g.leafNodes[:i], g.leafNodes[i+1:]...)
			break
		}
	}

	// If the to node has no children, it's a leaf
	if len(toNode.Children) == 0 {
		// Check if it's already in the leaf nodes
		found := false
		for _, leaf := range g.leafNodes {
			if leaf.ID == toNode.ID {
				found = true
				break
			}
		}
		if !found {
			g.leafNodes = append(g.leafNodes, toNode)
		}
	}

	return nil
}

// GetNode implements the ComputationGraph interface
func (g *inMemoryGraph) GetNode(ctx context.Context, id NodeID) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	if !exists {
		return nil, fmt.Errorf("%w: node %s not found", ErrNodeNotFound, id)
	}

	return node, nil
}

// GetNodeByPosition implements the ComputationGraph interface
func (g *inMemoryGraph) GetNodeByPosition(ctx context.Context, pos types.Position) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	key := positionKey(pos)
	node, exists := g.nodesByPos[key]
	if !exists {
		return nil, fmt.Errorf("%w: node at position %s not found", ErrNodeNotFound, pos)
	}

	return node, nil
}

// GetEdge implements the ComputationGraph interface
func (g *inMemoryGraph) GetEdge(ctx context.Context, from, to NodeID) (*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	key := edgeKey(from, to)
	edge, exists := g.edges[key]
	if !exists {
		return nil, fmt.Errorf("%w: edge from %s to %s not found", ErrInvalidEdge, from, to)
	}

	return edge, nil
}

// GetParents implements the ComputationGraph interface
func (g *inMemoryGraph) GetParents(ctx context.Context, id NodeID) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	if !exists {
		return nil, fmt.Errorf("%w: node %s not found", ErrNodeNotFound, id)
	}

	parents := make([]*Node, 0, len(node.Parents))
	for _, parentID := range node.Parents {
		parent, exists := g.nodes[parentID]
		if !exists {
			return nil, fmt.Errorf("%w: parent node %s not found", ErrNodeNotFound, parentID)
		}
		parents = append(parents, parent)
	}

	return parents, nil
}

// GetChildren implements the ComputationGraph interface
func (g *inMemoryGraph) GetChildren(ctx context.Context, id NodeID) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	if !exists {
		return nil, fmt.Errorf("%w: node %s not found", ErrNodeNotFound, id)
	}

	children := make([]*Node, 0, len(node.Children))
	for _, childID := range node.Children {
		child, exists := g.nodes[childID]
		if !exists {
			return nil, fmt.Errorf("%w: child node %s not found", ErrNodeNotFound, childID)
		}
		children = append(children, child)
	}

	return children, nil
}

// GetStepData implements the ComputationGraph interface
func (g *inMemoryGraph) GetStepData(ctx context.Context, from, to NodeID) ([]byte, *types.PreimageOracleData, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	key := edgeKey(from, to)
	edge, exists := g.edges[key]
	if !exists {
		return nil, nil, fmt.Errorf("%w: edge from %s to %s not found", ErrInvalidEdge, from, to)
	}

	// The proof data and preimage data should be stored in the edge metadata
	var proofData []byte
	var preimageData *types.PreimageOracleData

	if proofDataRaw, exists := edge.Metadata["proofData"]; exists {
		if pd, ok := proofDataRaw.([]byte); ok {
			proofData = pd
		}
	}

	if preimageDataRaw, exists := edge.Metadata["preimageData"]; exists {
		if pid, ok := preimageDataRaw.(*types.PreimageOracleData); ok {
			preimageData = pid
		}
	}

	return proofData, preimageData, nil
}

// GetRootNode implements the ComputationGraph interface
func (g *inMemoryGraph) GetRootNode(ctx context.Context) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.rootNode == nil {
		return nil, fmt.Errorf("%w: root node not found", ErrGraphIncomplete)
	}

	return g.rootNode, nil
}

// GetLeafNodes implements the ComputationGraph interface
func (g *inMemoryGraph) GetLeafNodes(ctx context.Context) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.leafNodes) == 0 {
		return nil, fmt.Errorf("%w: no leaf nodes found", ErrGraphIncomplete)
	}

	// Return a copy of the leaf nodes slice to prevent modification
	leafNodes := make([]*Node, len(g.leafNodes))
	copy(leafNodes, g.leafNodes)

	return leafNodes, nil
}

// GetPath implements the ComputationGraph interface
func (g *inMemoryGraph) GetPath(ctx context.Context, start, end NodeID) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Check if both nodes exist
	_, exists := g.nodes[start]
	if !exists {
		return nil, fmt.Errorf("%w: start node %s not found", ErrNodeNotFound, start)
	}

	_, exists = g.nodes[end]
	if !exists {
		return nil, fmt.Errorf("%w: end node %s not found", ErrNodeNotFound, end)
	}

	// Use BFS to find the shortest path
	visited := make(map[NodeID]bool)
	queue := []NodeID{start}
	prev := make(map[NodeID]NodeID)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == end {
			// Found the end, reconstruct the path
			path := []*Node{g.nodes[end]}
			for current != start {
				current = prev[current]
				path = append([]*Node{g.nodes[current]}, path...)
			}
			return path, nil
		}

		visited[current] = true

		// Add all unvisited children to the queue
		for _, childID := range g.nodes[current].Children {
			if !visited[childID] {
				queue = append(queue, childID)
				prev[childID] = current
			}
		}
	}

	return nil, fmt.Errorf("no path found from %s to %s", start, end)
}

// GetNodeCount implements the ComputationGraph interface
func (g *inMemoryGraph) GetNodeCount(ctx context.Context) (uint64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return uint64(len(g.nodes)), nil
}

// GetEdgeCount implements the ComputationGraph interface
func (g *inMemoryGraph) GetEdgeCount(ctx context.Context) (uint64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return uint64(len(g.edges)), nil
}

// AddOperation implements the ComputationGraph interface
func (g *inMemoryGraph) AddOperation(ctx context.Context, op Operation) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Validate the operation
	if op.ID() == "" {
		return fmt.Errorf("%w: operation ID cannot be empty", ErrInvalidOperation)
	}

	// Check if the operation already exists
	if _, exists := g.operations[op.ID()]; exists {
		return fmt.Errorf("%w: operation with ID %s already exists", ErrInvalidOperation, op.ID())
	}

	// Add the operation to the map
	g.operations[op.ID()] = op

	return nil
}

// GetOperations implements the ComputationGraph interface
func (g *inMemoryGraph) GetOperations(ctx context.Context) ([]Operation, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	operations := make([]Operation, 0, len(g.operations))
	for _, op := range g.operations {
		operations = append(operations, op)
	}

	return operations, nil
}

// GetOperationByID implements the ComputationGraph interface
func (g *inMemoryGraph) GetOperationByID(ctx context.Context, id string) (Operation, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	op, exists := g.operations[id]
	if !exists {
		return nil, fmt.Errorf("%w: operation with ID %s not found", ErrInvalidOperation, id)
	}

	return op, nil
}

// Ensure inMemoryGraph implements ComputationGraph
var _ ComputationGraph = (*inMemoryGraph)(nil)

// BuildTraceGraph builds a computation graph from a trace provider
// This is a helper function that can be used by specific implementations
func BuildTraceGraph(
	ctx context.Context,
	logger log.Logger,
	traceProvider types.TraceProvider,
	gameDepth types.Depth,
	nodeBuilder func(pos types.Position, value common.Hash, data []byte) (*Node, error),
	edgeBuilder func(fromNode, toNode *Node, fromPos, toPos types.Position) (*Edge, error),
) (ComputationGraph, error) {
	graph := newInMemoryGraph()

	// Get the absolute prestate commitment
	absolutePrestate, err := traceProvider.AbsolutePreStateCommitment(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute prestate commitment: %w", err)
	}

	// Create the root node
	rootPos := types.RootPosition
	rootValue, err := traceProvider.Get(ctx, rootPos)
	if err != nil {
		return nil, fmt.Errorf("failed to get root value: %w", err)
	}

	// Get the root node's data
	rootPrestate, _, _, err := traceProvider.GetStepData(ctx, rootPos)
	if err != nil {
		return nil, fmt.Errorf("failed to get root step data: %w", err)
	}

	rootNode, err := nodeBuilder(rootPos, rootValue, rootPrestate)
	if err != nil {
		return nil, fmt.Errorf("failed to build root node: %w", err)
	}

	// Add metadata about the absolute prestate
	if rootNode.Metadata == nil {
		rootNode.Metadata = make(map[string]interface{})
	}
	rootNode.Metadata["absolutePrestate"] = absolutePrestate

	graph.AddNodeWithPosition(rootNode, rootPos)

	// Build the rest of the graph
	// This is a simplified approach that builds a linear trace
	// More complex implementations might build a more structured graph

	// Start from the root and build the trace
	currentPos := rootPos
	currentNode := rootNode

	// Build until we reach the maximum depth
	for depth := uint64(1); depth <= uint64(gameDepth); depth++ {
		// Get the next position
		nextPos := currentPos.Attack()

		// Get the value at this position
		nextValue, err := traceProvider.Get(ctx, nextPos)
		if err != nil {
			logger.Warn("Failed to get value at position", "position", nextPos, "err", err)
			break
		}

		// Get the step data
		nextPrestate, proofData, preimageData, err := traceProvider.GetStepData(ctx, nextPos)
		if err != nil {
			logger.Warn("Failed to get step data at position", "position", nextPos, "err", err)
			break
		}

		// Build the next node
		nextNode, err := nodeBuilder(nextPos, nextValue, nextPrestate)
		if err != nil {
			logger.Warn("Failed to build node at position", "position", nextPos, "err", err)
			break
		}

		graph.AddNodeWithPosition(nextNode, nextPos)

		// Build the edge between the current and next node
		edge, err := edgeBuilder(currentNode, nextNode, currentPos, nextPos)
		if err != nil {
			logger.Warn("Failed to build edge between positions", "from", currentPos, "to", nextPos, "err", err)
			break
		}

		// Add the proof data and preimage data to the edge metadata
		if edge.Metadata == nil {
			edge.Metadata = make(map[string]interface{})
		}
		edge.Metadata["proofData"] = proofData
		edge.Metadata["preimageData"] = preimageData

		if err := graph.AddEdge(edge); err != nil {
			logger.Warn("Failed to add edge to graph", "err", err)
			break
		}

		// Move to the next position
		currentPos = nextPos
		currentNode = nextNode
	}

	return graph, nil
}

// BuildPartialTraceGraph builds a partial computation graph around a specific position
func BuildPartialTraceGraph(
	ctx context.Context,
	logger log.Logger,
	traceProvider types.TraceProvider,
	pos types.Position,
	depth uint64,
	gameDepth types.Depth,
	nodeBuilder func(pos types.Position, value common.Hash, data []byte) (*Node, error),
	edgeBuilder func(fromNode, toNode *Node, fromPos, toPos types.Position) (*Edge, error),
) (ComputationGraph, error) {
	graph := newInMemoryGraph()

	// Get the value at the specified position
	value, err := traceProvider.Get(ctx, pos)
	if err != nil {
		return nil, fmt.Errorf("failed to get value at position %s: %w", pos, err)
	}

	// Get the step data
	prestate, _, _, err := traceProvider.GetStepData(ctx, pos)
	if err != nil {
		return nil, fmt.Errorf("failed to get step data at position %s: %w", pos, err)
	}

	// Build the node for the specified position
	node, err := nodeBuilder(pos, value, prestate)
	if err != nil {
		return nil, fmt.Errorf("failed to build node at position %s: %w", pos, err)
	}

	graph.AddNodeWithPosition(node, pos)

	// Build nodes for parents up to the specified depth
	currentPos := pos
	currentNode := node

	// Build parent nodes
	for i := uint64(0); i < depth && !currentPos.IsRootPosition(); i++ {
		parentPos := currentPos.Parent()

		// Get the value at the parent position
		parentValue, err := traceProvider.Get(ctx, parentPos)
		if err != nil {
			logger.Warn("Failed to get value at parent position", "position", parentPos, "err", err)
			break
		}

		// Get the step data
		parentPrestate, proofData, preimageData, err := traceProvider.GetStepData(ctx, parentPos)
		if err != nil {
			logger.Warn("Failed to get step data at parent position", "position", parentPos, "err", err)
			break
		}

		// Build the parent node
		parentNode, err := nodeBuilder(parentPos, parentValue, parentPrestate)
		if err != nil {
			logger.Warn("Failed to build node at parent position", "position", parentPos, "err", err)
			break
		}

		graph.AddNodeWithPosition(parentNode, parentPos)

		// Build the edge between the parent and current node
		edge, err := edgeBuilder(parentNode, currentNode, parentPos, currentPos)
		if err != nil {
			logger.Warn("Failed to build edge between positions", "from", parentPos, "to", currentPos, "err", err)
			break
		}

		// Add the proof data and preimage data to the edge metadata
		if edge.Metadata == nil {
			edge.Metadata = make(map[string]interface{})
		}
		edge.Metadata["proofData"] = proofData
		edge.Metadata["preimageData"] = preimageData

		if err := graph.AddEdge(edge); err != nil {
			logger.Warn("Failed to add edge to graph", "err", err)
			break
		}

		// Move to the parent position
		currentPos = parentPos
		currentNode = parentNode
	}

	// Build nodes for children up to the specified depth
	// This is a simplified approach that only builds the attack child
	// More complex implementations might build all children

	currentPos = pos
	currentNode = node

	// Build child nodes
	for i := uint64(0); i < depth && currentPos.Depth() < gameDepth; i++ {
		childPos := currentPos.Attack()

		// Get the value at the child position
		childValue, err := traceProvider.Get(ctx, childPos)
		if err != nil {
			logger.Warn("Failed to get value at child position", "position", childPos, "err", err)
			break
		}

		// Get the step data
		childPrestate, proofData, preimageData, err := traceProvider.GetStepData(ctx, childPos)
		if err != nil {
			logger.Warn("Failed to get step data at child position", "position", childPos, "err", err)
			break
		}

		// Build the child node
		childNode, err := nodeBuilder(childPos, childValue, childPrestate)
		if err != nil {
			logger.Warn("Failed to build node at child position", "position", childPos, "err", err)
			break
		}

		graph.AddNodeWithPosition(childNode, childPos)

		// Build the edge between the current and child node
		edge, err := edgeBuilder(currentNode, childNode, currentPos, childPos)
		if err != nil {
			logger.Warn("Failed to build edge between positions", "from", currentPos, "to", childPos, "err", err)
			break
		}

		// Add the proof data and preimage data to the edge metadata
		if edge.Metadata == nil {
			edge.Metadata = make(map[string]interface{})
		}
		edge.Metadata["proofData"] = proofData
		edge.Metadata["preimageData"] = preimageData

		if err := graph.AddEdge(edge); err != nil {
			logger.Warn("Failed to add edge to graph", "err", err)
			break
		}

		// Move to the child position
		currentPos = childPos
		currentNode = childNode
	}

	return graph, nil
}
