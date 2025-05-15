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

// Package graph provides a computation graph implementation for fault dispute games.
// The computation graph represents the execution trace as a directed graph where
// nodes are states and edges are transitions between states.
package graph

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/common"
)

var (
	// ErrNodeNotFound is returned when a node is not found in the computation graph
	ErrNodeNotFound = errors.New("node not found in computation graph")
	// ErrInvalidEdge is returned when an edge between nodes is invalid
	ErrInvalidEdge = errors.New("invalid edge between nodes")
	// ErrGraphIncomplete is returned when the computation graph is incomplete
	ErrGraphIncomplete = errors.New("computation graph is incomplete")
	// ErrInvalidOperation is returned when an operation is invalid
	ErrInvalidOperation = errors.New("invalid operation")
	// ErrInvalidPort is returned when a port is invalid
	ErrInvalidPort = errors.New("invalid port")
	// ErrInvalidTensor is returned when a tensor is invalid
	ErrInvalidTensor = errors.New("invalid tensor")
)

// NodeID uniquely identifies a node in the computation graph
type NodeID string

// Node represents a single node in the computation graph.
// Each node corresponds to a state in the execution trace.
type Node struct {
	// ID is the unique identifier for this node
	ID NodeID
	// Value is the hash value of this node's state
	Value common.Hash
	// Data contains the raw state data for this node
	Data []byte
	// Parents are the IDs of the nodes that are direct parents of this node
	Parents []NodeID
	// Children are the IDs of the nodes that are direct children of this node
	Children []NodeID
	// Metadata contains additional information about this node
	Metadata map[string]interface{}
}

// Edge represents a directed edge between two nodes in the computation graph.
// Edges represent transitions between states in the execution trace.
type Edge struct {
	// From is the ID of the source node
	From NodeID
	// To is the ID of the destination node
	To NodeID
	// Type describes the type of relationship between the nodes
	Type string
	// Metadata contains additional information about this edge
	Metadata map[string]interface{}
}

// ComputationGraph represents a directed graph of computation steps
// where nodes are states and edges are transitions between states.
// The graph provides methods to traverse and query the execution trace.
type ComputationGraph interface {
	// GetNode returns the node with the given ID.
	// Returns ErrNodeNotFound if the node does not exist.
	GetNode(ctx context.Context, id NodeID) (*Node, error)

	// GetNodeByPosition returns the node at the given position in the trace.
	// This allows mapping between the linear trace position and the graph structure.
	// Returns ErrNodeNotFound if no node exists at the given position.
	GetNodeByPosition(ctx context.Context, pos types.Position) (*Node, error)

	// GetEdge returns the edge between the two nodes.
	// Returns ErrInvalidEdge if no edge exists between the nodes.
	GetEdge(ctx context.Context, from, to NodeID) (*Edge, error)

	// GetParents returns all parent nodes of the given node.
	// Returns ErrNodeNotFound if the node does not exist.
	GetParents(ctx context.Context, id NodeID) ([]*Node, error)

	// GetChildren returns all child nodes of the given node.
	// Returns ErrNodeNotFound if the node does not exist.
	GetChildren(ctx context.Context, id NodeID) ([]*Node, error)

	// GetStepData returns the data required to execute the step from one node to another.
	// This includes the proof data and any preimage data needed for the transition.
	// Returns ErrInvalidEdge if no edge exists between the nodes.
	GetStepData(ctx context.Context, from, to NodeID) (proofData []byte, preimageData *types.PreimageOracleData, err error)

	// GetRootNode returns the root node of the computation graph.
	// The root node represents the initial state of the execution trace.
	GetRootNode(ctx context.Context) (*Node, error)

	// GetLeafNodes returns all leaf nodes of the computation graph.
	// Leaf nodes represent terminal states in the execution trace.
	GetLeafNodes(ctx context.Context) ([]*Node, error)

	// GetPath returns a path from the start node to the end node.
	// Returns ErrNodeNotFound if either node does not exist.
	// Returns ErrGraphIncomplete if no path exists between the nodes.
	GetPath(ctx context.Context, start, end NodeID) ([]*Node, error)

	// GetNodeCount returns the total number of nodes in the graph.
	GetNodeCount(ctx context.Context) (uint64, error)

	// GetEdgeCount returns the total number of edges in the graph.
	GetEdgeCount(ctx context.Context) (uint64, error)

	// AddOperation adds an operation to the graph.
	// Returns ErrInvalidOperation if the operation is invalid.
	AddOperation(ctx context.Context, op Operation) error

	// GetOperations returns all operations in the graph.
	GetOperations(ctx context.Context) ([]Operation, error)

	// GetOperationByID returns the operation with the given ID.
	// Returns ErrInvalidOperation if the operation does not exist.
	GetOperationByID(ctx context.Context, id string) (Operation, error)
}

// GraphBuilder is responsible for constructing a computation graph.
// It provides methods to build a complete graph or a partial graph
// around a specific position in the trace.
type GraphBuilder interface {
	// BuildGraph builds a computation graph from the given trace.
	// The resulting graph represents the entire execution trace up to the specified game depth.
	BuildGraph(ctx context.Context, traceProvider types.TraceProvider, gameDepth types.Depth) (ComputationGraph, error)

	// BuildPartialGraph builds a partial computation graph around the given position.
	// This is useful for optimizing resource usage when only a portion of the graph is needed.
	// The depth parameter controls how many levels of the graph to build around the position.
	BuildPartialGraph(ctx context.Context, traceProvider types.TraceProvider, pos types.Position, depth uint64, gameDepth types.Depth) (ComputationGraph, error)
}

// PortDirection represents the direction of a port (input or output)
type PortDirection int

const (
	// PortDirectionInput represents an input port
	PortDirectionInput PortDirection = iota
	// PortDirectionOutput represents an output port
	PortDirectionOutput
)

// Port represents a connection point for an operation.
// Ports are used to connect operations together in the computation graph.
type Port interface {
	// ID returns the unique identifier for this port
	ID() string

	// Name returns the human-readable name of this port
	Name() string

	// Direction returns the direction of this port (input or output)
	Direction() PortDirection

	// Operation returns the operation that this port belongs to
	Operation() Operation

	// ConnectedTo returns the ports that this port is connected to
	ConnectedTo() []Port

	// Connect connects this port to another port
	Connect(port Port) error

	// Disconnect disconnects this port from another port
	Disconnect(port Port) error

	// Tensor returns the tensor associated with this port
	Tensor() Tensor

	// SetTensor sets the tensor for this port
	SetTensor(tensor Tensor) error
}

// Operation represents a computational operation in the graph.
// Operations are connected together via ports to form the computation graph.
type Operation interface {
	// ID returns the unique identifier for this operation
	ID() string

	// Name returns the human-readable name of this operation
	Name() string

	// Type returns the type of this operation
	Type() string

	// InputPorts returns the input ports for this operation
	InputPorts() []Port

	// OutputPorts returns the output ports for this operation
	OutputPorts() []Port

	// GetPort returns the port with the given name
	GetPort(name string) (Port, error)

	// Execute executes this operation with the given inputs and returns the outputs
	Execute(ctx context.Context) error

	// Metadata returns additional information about this operation
	Metadata() map[string]interface{}

	// SetMetadata sets additional information about this operation
	SetMetadata(metadata map[string]interface{})
}

// TensorDataType represents the data type of a tensor
type TensorDataType int

const (
	// TensorDataTypeUnknown represents an unknown data type
	TensorDataTypeUnknown TensorDataType = iota
	// TensorDataTypeFloat32 represents a 32-bit floating point data type
	TensorDataTypeFloat32
	// TensorDataTypeFloat64 represents a 64-bit floating point data type
	TensorDataTypeFloat64
	// TensorDataTypeInt32 represents a 32-bit integer data type
	TensorDataTypeInt32
	// TensorDataTypeInt64 represents a 64-bit integer data type
	TensorDataTypeInt64
	// TensorDataTypeUint8 represents an 8-bit unsigned integer data type
	TensorDataTypeUint8
	// TensorDataTypeBytes represents a byte array data type
	TensorDataTypeBytes
	// TensorDataTypeHash represents a hash data type
	TensorDataTypeHash
)

// Tensor represents a multi-dimensional array of data.
// Tensors are used to store the inputs and outputs of operations.
type Tensor interface {
	// ID returns the unique identifier for this tensor
	ID() string

	// Name returns the human-readable name of this tensor
	Name() string

	// Shape returns the shape of this tensor
	Shape() []int

	// DataType returns the data type of this tensor
	DataType() TensorDataType

	// Data returns the raw data of this tensor
	Data() interface{}

	// SetData sets the raw data of this tensor
	SetData(data interface{}) error

	// Hash returns a hash of this tensor's data
	Hash() common.Hash

	// Metadata returns additional information about this tensor
	Metadata() map[string]interface{}

	// SetMetadata sets additional information about this tensor
	SetMetadata(metadata map[string]interface{})
}
