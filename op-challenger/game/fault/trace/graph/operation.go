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
)

// BaseOperation provides a basic implementation of the Operation interface.
// It can be embedded in more specific operation implementations.
type BaseOperation struct {
	id          string
	name        string
	opType      string
	inputPorts  []Port
	outputPorts []Port
	metadata    map[string]interface{}
	mu          sync.RWMutex
}

// NewBaseOperation creates a new BaseOperation with the given ID, name, and type.
func NewBaseOperation(id, name, opType string) *BaseOperation {
	return &BaseOperation{
		id:          id,
		name:        name,
		opType:      opType,
		inputPorts:  make([]Port, 0),
		outputPorts: make([]Port, 0),
		metadata:    make(map[string]interface{}),
	}
}

// ID implements the Operation interface.
func (o *BaseOperation) ID() string {
	return o.id
}

// Name implements the Operation interface.
func (o *BaseOperation) Name() string {
	return o.name
}

// Type implements the Operation interface.
func (o *BaseOperation) Type() string {
	return o.opType
}

// InputPorts implements the Operation interface.
func (o *BaseOperation) InputPorts() []Port {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Return a copy of the input ports slice to prevent modification
	ports := make([]Port, len(o.inputPorts))
	copy(ports, o.inputPorts)
	return ports
}

// OutputPorts implements the Operation interface.
func (o *BaseOperation) OutputPorts() []Port {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Return a copy of the output ports slice to prevent modification
	ports := make([]Port, len(o.outputPorts))
	copy(ports, o.outputPorts)
	return ports
}

// GetPort implements the Operation interface.
func (o *BaseOperation) GetPort(name string) (Port, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Check input ports
	for _, port := range o.inputPorts {
		if port.Name() == name {
			return port, nil
		}
	}

	// Check output ports
	for _, port := range o.outputPorts {
		if port.Name() == name {
			return port, nil
		}
	}

	return nil, fmt.Errorf("%w: port %s not found", ErrInvalidPort, name)
}

// AddInputPort adds an input port to the operation.
func (o *BaseOperation) AddInputPort(port Port) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.inputPorts = append(o.inputPorts, port)
}

// AddOutputPort adds an output port to the operation.
func (o *BaseOperation) AddOutputPort(port Port) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.outputPorts = append(o.outputPorts, port)
}

// Execute implements the Operation interface.
// This is a base implementation that should be overridden by specific operation types.
func (o *BaseOperation) Execute(ctx context.Context) error {
	// Base implementation does nothing
	// Specific operation types should override this method
	return nil
}

// Metadata implements the Operation interface.
func (o *BaseOperation) Metadata() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Return a copy of the metadata map to prevent modification
	metadata := make(map[string]interface{})
	for k, v := range o.metadata {
		metadata[k] = v
	}
	return metadata
}

// SetMetadata implements the Operation interface.
func (o *BaseOperation) SetMetadata(metadata map[string]interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.metadata = metadata
}

// BasePort provides a basic implementation of the Port interface.
// It can be embedded in more specific port implementations.
type BasePort struct {
	id        string
	name      string
	direction PortDirection
	operation Operation
	connected []Port
	tensor    Tensor
	mu        sync.RWMutex
}

// NewBasePort creates a new BasePort with the given ID, name, direction, and operation.
func NewBasePort(id, name string, direction PortDirection, operation Operation) *BasePort {
	return &BasePort{
		id:        id,
		name:      name,
		direction: direction,
		operation: operation,
		connected: make([]Port, 0),
	}
}

// ID implements the Port interface.
func (p *BasePort) ID() string {
	return p.id
}

// Name implements the Port interface.
func (p *BasePort) Name() string {
	return p.name
}

// Direction implements the Port interface.
func (p *BasePort) Direction() PortDirection {
	return p.direction
}

// Operation implements the Port interface.
func (p *BasePort) Operation() Operation {
	return p.operation
}

// ConnectedTo implements the Port interface.
func (p *BasePort) ConnectedTo() []Port {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy of the connected ports slice to prevent modification
	ports := make([]Port, len(p.connected))
	copy(ports, p.connected)
	return ports
}

// Connect implements the Port interface.
func (p *BasePort) Connect(port Port) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if the ports are compatible
	if p.direction == port.Direction() {
		return fmt.Errorf("%w: cannot connect ports with the same direction", ErrInvalidPort)
	}

	// Check if the ports are already connected
	for _, connectedPort := range p.connected {
		if connectedPort.ID() == port.ID() {
			// Already connected
			return nil
		}
	}

	// Connect the ports
	p.connected = append(p.connected, port)

	// Connect the other port to this port
	// This is done outside the lock to avoid deadlocks
	p.mu.Unlock()
	err := port.Connect(p)
	p.mu.Lock()

	return err
}

// Disconnect implements the Port interface.
func (p *BasePort) Disconnect(port Port) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find the port in the connected ports
	for i, connectedPort := range p.connected {
		if connectedPort.ID() == port.ID() {
			// Remove the port from the connected ports
			p.connected = append(p.connected[:i], p.connected[i+1:]...)

			// Disconnect the other port from this port
			// This is done outside the lock to avoid deadlocks
			p.mu.Unlock()
			err := port.Disconnect(p)
			p.mu.Lock()

			return err
		}
	}

	// Port not found
	return fmt.Errorf("%w: port %s not connected", ErrInvalidPort, port.ID())
}

// Tensor implements the Port interface.
func (p *BasePort) Tensor() Tensor {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.tensor
}

// SetTensor implements the Port interface.
func (p *BasePort) SetTensor(tensor Tensor) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.tensor = tensor
	return nil
}

// Ensure BaseOperation implements Operation
var _ Operation = (*BaseOperation)(nil)

// Ensure BasePort implements Port
var _ Port = (*BasePort)(nil)
