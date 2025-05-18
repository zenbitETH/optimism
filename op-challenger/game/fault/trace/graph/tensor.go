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
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// BaseTensor provides a basic implementation of the Tensor interface.
// It can be embedded in more specific tensor implementations.
type BaseTensor struct {
	id       string
	name     string
	shape    []int
	dataType TensorDataType
	data     interface{}
	metadata map[string]interface{}
	mu       sync.RWMutex
}

// NewBaseTensor creates a new BaseTensor with the given ID, name, shape, and data type.
func NewBaseTensor(id, name string, shape []int, dataType TensorDataType) *BaseTensor {
	return &BaseTensor{
		id:       id,
		name:     name,
		shape:    shape,
		dataType: dataType,
		metadata: make(map[string]interface{}),
	}
}

// ID implements the Tensor interface.
func (t *BaseTensor) ID() string {
	return t.id
}

// Name implements the Tensor interface.
func (t *BaseTensor) Name() string {
	return t.name
}

// Shape implements the Tensor interface.
func (t *BaseTensor) Shape() []int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Return a copy of the shape slice to prevent modification
	shape := make([]int, len(t.shape))
	copy(shape, t.shape)
	return shape
}

// DataType implements the Tensor interface.
func (t *BaseTensor) DataType() TensorDataType {
	return t.dataType
}

// Data implements the Tensor interface.
func (t *BaseTensor) Data() interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.data
}

// SetData implements the Tensor interface.
func (t *BaseTensor) SetData(data interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate the data type
	switch t.dataType {
	case TensorDataTypeFloat32:
		if _, ok := data.([]float32); !ok {
			return fmt.Errorf("%w: expected []float32 for TensorDataTypeFloat32", ErrInvalidTensor)
		}
	case TensorDataTypeFloat64:
		if _, ok := data.([]float64); !ok {
			return fmt.Errorf("%w: expected []float64 for TensorDataTypeFloat64", ErrInvalidTensor)
		}
	case TensorDataTypeInt32:
		if _, ok := data.([]int32); !ok {
			return fmt.Errorf("%w: expected []int32 for TensorDataTypeInt32", ErrInvalidTensor)
		}
	case TensorDataTypeInt64:
		if _, ok := data.([]int64); !ok {
			return fmt.Errorf("%w: expected []int64 for TensorDataTypeInt64", ErrInvalidTensor)
		}
	case TensorDataTypeUint8:
		if _, ok := data.([]uint8); !ok {
			return fmt.Errorf("%w: expected []uint8 for TensorDataTypeUint8", ErrInvalidTensor)
		}
	case TensorDataTypeBytes:
		if _, ok := data.([]byte); !ok {
			return fmt.Errorf("%w: expected []byte for TensorDataTypeBytes", ErrInvalidTensor)
		}
	case TensorDataTypeHash:
		if _, ok := data.(common.Hash); !ok {
			return fmt.Errorf("%w: expected common.Hash for TensorDataTypeHash", ErrInvalidTensor)
		}
	default:
		// For unknown data types, accept any data
	}

	t.data = data
	return nil
}

// Hash implements the Tensor interface.
func (t *BaseTensor) Hash() common.Hash {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Hash the data based on its type
	switch t.dataType {
	case TensorDataTypeFloat32:
		if data, ok := t.data.([]float32); ok {
			// Convert to bytes and hash
			bytes := make([]byte, len(data)*4)
			for i, v := range data {
				// Simple conversion for demonstration
				// In a real implementation, this would use binary.LittleEndian.PutUint32
				bytes[i*4] = byte(int(v) & 0xFF)
				bytes[i*4+1] = byte((int(v) >> 8) & 0xFF)
				bytes[i*4+2] = byte((int(v) >> 16) & 0xFF)
				bytes[i*4+3] = byte((int(v) >> 24) & 0xFF)
			}
			return crypto.Keccak256Hash(bytes)
		}
	case TensorDataTypeFloat64:
		if data, ok := t.data.([]float64); ok {
			// Convert to bytes and hash
			bytes := make([]byte, len(data)*8)
			for i, v := range data {
				// Simple conversion for demonstration
				// In a real implementation, this would use binary.LittleEndian.PutUint64
				bytes[i*8] = byte(int(v) & 0xFF)
				bytes[i*8+1] = byte((int(v) >> 8) & 0xFF)
				bytes[i*8+2] = byte((int(v) >> 16) & 0xFF)
				bytes[i*8+3] = byte((int(v) >> 24) & 0xFF)
				bytes[i*8+4] = byte((int(v) >> 32) & 0xFF)
				bytes[i*8+5] = byte((int(v) >> 40) & 0xFF)
				bytes[i*8+6] = byte((int(v) >> 48) & 0xFF)
				bytes[i*8+7] = byte((int(v) >> 56) & 0xFF)
			}
			return crypto.Keccak256Hash(bytes)
		}
	case TensorDataTypeInt32:
		if data, ok := t.data.([]int32); ok {
			// Convert to bytes and hash
			bytes := make([]byte, len(data)*4)
			for i, v := range data {
				bytes[i*4] = byte(v & 0xFF)
				bytes[i*4+1] = byte((v >> 8) & 0xFF)
				bytes[i*4+2] = byte((v >> 16) & 0xFF)
				bytes[i*4+3] = byte((v >> 24) & 0xFF)
			}
			return crypto.Keccak256Hash(bytes)
		}
	case TensorDataTypeInt64:
		if data, ok := t.data.([]int64); ok {
			// Convert to bytes and hash
			bytes := make([]byte, len(data)*8)
			for i, v := range data {
				bytes[i*8] = byte(v & 0xFF)
				bytes[i*8+1] = byte((v >> 8) & 0xFF)
				bytes[i*8+2] = byte((v >> 16) & 0xFF)
				bytes[i*8+3] = byte((v >> 24) & 0xFF)
				bytes[i*8+4] = byte((v >> 32) & 0xFF)
				bytes[i*8+5] = byte((v >> 40) & 0xFF)
				bytes[i*8+6] = byte((v >> 48) & 0xFF)
				bytes[i*8+7] = byte((v >> 56) & 0xFF)
			}
			return crypto.Keccak256Hash(bytes)
		}
	case TensorDataTypeUint8:
		if data, ok := t.data.([]uint8); ok {
			return crypto.Keccak256Hash(data)
		}
	case TensorDataTypeBytes:
		if data, ok := t.data.([]byte); ok {
			return crypto.Keccak256Hash(data)
		}
	case TensorDataTypeHash:
		if data, ok := t.data.(common.Hash); ok {
			return data
		}
	}

	// Default: serialize the data to a string and hash it
	return crypto.Keccak256Hash([]byte(fmt.Sprintf("%v", t.data)))
}

// Metadata implements the Tensor interface.
func (t *BaseTensor) Metadata() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Return a copy of the metadata map to prevent modification
	metadata := make(map[string]interface{})
	for k, v := range t.metadata {
		metadata[k] = v
	}
	return metadata
}

// SetMetadata implements the Tensor interface.
func (t *BaseTensor) SetMetadata(metadata map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.metadata = metadata
}

// Ensure BaseTensor implements Tensor
var _ Tensor = (*BaseTensor)(nil)
