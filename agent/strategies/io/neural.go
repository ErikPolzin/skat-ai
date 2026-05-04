package io

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/storage"
	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
)

// LoadBiddingWeights loads bidding network weights from a binary file directly into nodes
func LoadBiddingWeights(path string, g *gorgonia.ExprGraph) (map[string]*gorgonia.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, err
	}
	if string(magic) != "SKAT" {
		return nil, fmt.Errorf("invalid magic number")
	}

	// Read version
	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return nil, err
	}
	if version != 1 && version != 2 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	weights := make(map[string]*gorgonia.Node)

	// Read each tensor and create nodes directly
	for {
		// Read name length
		var nameLen uint32
		if err := binary.Read(f, binary.LittleEndian, &nameLen); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Read name
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(f, nameBytes); err != nil {
			return nil, err
		}
		name := string(nameBytes)

		// Read shape
		var ndim uint32
		if err := binary.Read(f, binary.LittleEndian, &ndim); err != nil {
			return nil, err
		}

		shape := make([]int, ndim)
		for i := range shape {
			var dim uint32
			if err := binary.Read(f, binary.LittleEndian, &dim); err != nil {
				return nil, err
			}
			shape[i] = int(dim)
		}

		// Calculate total elements
		totalElems := 1
		for _, dim := range shape {
			totalElems *= dim
		}

		// Read data
		data := make([]float32, totalElems)
		if err := binary.Read(f, binary.LittleEndian, &data); err != nil {
			return nil, err
		}

		// Create node with data
		var node *gorgonia.Node
		if len(shape) == 1 {
			node = gorgonia.NewVector(g, tensor.Float32, gorgonia.WithShape(shape...), gorgonia.WithName(name))
		} else {
			node = gorgonia.NewMatrix(g, tensor.Float32, gorgonia.WithShape(shape...), gorgonia.WithName(name))
		}

		// Set the value
		t := tensor.New(tensor.WithBacking(data), tensor.WithShape(shape...))
		gorgonia.Let(node, t)

		weights[name] = node
	}

	return weights, nil
}

// LoadCardPlayWeights loads card play network weights from binary file
func LoadCardPlayWeights(path string, g *gorgonia.ExprGraph) (map[string]*gorgonia.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, err
	}
	if string(magic) != "SKAT" {
		return nil, fmt.Errorf("invalid magic number")
	}

	// Read version
	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return nil, err
	}

	weights := make(map[string]*gorgonia.Node)

	// Read each tensor and create nodes
	for {
		var nameLen uint32
		if err := binary.Read(f, binary.LittleEndian, &nameLen); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(f, nameBytes); err != nil {
			return nil, err
		}
		name := string(nameBytes)

		var ndim uint32
		if err := binary.Read(f, binary.LittleEndian, &ndim); err != nil {
			return nil, err
		}

		shape := make([]int, ndim)
		for i := range shape {
			var dim uint32
			if err := binary.Read(f, binary.LittleEndian, &dim); err != nil {
				return nil, err
			}
			shape[i] = int(dim)
		}

		totalElems := 1
		for _, dim := range shape {
			totalElems *= dim
		}

		data := make([]float32, totalElems)
		if err := binary.Read(f, binary.LittleEndian, &data); err != nil {
			return nil, err
		}

		// Create tensor and node
		t := tensor.New(tensor.WithBacking(data), tensor.WithShape(shape...))
		node := gorgonia.NodeFromAny(g, t, gorgonia.WithName(name))
		weights[name] = node
	}

	return weights, nil
}

// LoadCardPlayWeightsFromGCS loads card play network weights from GCS directly into nodes
func LoadCardPlayWeightsFromGCS(ctx context.Context, bucketName, objectName string, g *gorgonia.ExprGraph) (map[string]*gorgonia.Node, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS reader: %w", err)
	}
	defer reader.Close()

	// Read magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(reader, magic); err != nil {
		return nil, err
	}
	if string(magic) != "SKAT" {
		return nil, fmt.Errorf("invalid magic number")
	}

	// Read version
	var version uint32
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil {
		return nil, err
	}
	if version != 2 {
		return nil, fmt.Errorf("unsupported version: %d (expected 2)", version)
	}

	weights := make(map[string]*gorgonia.Node)

	// Read weights for each layer
	for {
		var nameLen uint32
		err := binary.Read(reader, binary.LittleEndian, &nameLen)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(reader, nameBytes); err != nil {
			return nil, err
		}
		name := string(nameBytes)

		var ndim uint32
		if err := binary.Read(reader, binary.LittleEndian, &ndim); err != nil {
			return nil, err
		}

		shape := make([]int, ndim)
		for i := range shape {
			var dim uint32
			if err := binary.Read(reader, binary.LittleEndian, &dim); err != nil {
				return nil, err
			}
			shape[i] = int(dim)
		}

		totalElems := 1
		for _, dim := range shape {
			totalElems *= dim
		}

		data := make([]float32, totalElems)
		if err := binary.Read(reader, binary.LittleEndian, &data); err != nil {
			return nil, err
		}

		// Create tensor and node
		t := tensor.New(tensor.WithBacking(data), tensor.WithShape(shape...))
		node := gorgonia.NodeFromAny(g, t, gorgonia.WithName(name))
		weights[name] = node
	}

	return weights, nil
}

// SaveWeights saves network weights to a binary file
func SaveWeights(path string, weights map[string]*gorgonia.Node) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write magic number
	if _, err := f.Write([]byte("SKAT")); err != nil {
		return err
	}

	// Write version
	version := uint32(2)
	if err := binary.Write(f, binary.LittleEndian, version); err != nil {
		return err
	}

	// Write each tensor
	for name, node := range weights {
		// Write name length
		nameLen := uint32(len(name))
		if err := binary.Write(f, binary.LittleEndian, nameLen); err != nil {
			return err
		}

		// Write name
		if _, err := f.Write([]byte(name)); err != nil {
			return err
		}

		// Write shape
		shape := node.Shape()
		ndim := uint32(len(shape))
		if err := binary.Write(f, binary.LittleEndian, ndim); err != nil {
			return err
		}

		for _, dim := range shape {
			if err := binary.Write(f, binary.LittleEndian, uint32(dim)); err != nil {
				return err
			}
		}

		// Write data
		data := node.Value().Data().([]float32)
		if err := binary.Write(f, binary.LittleEndian, data); err != nil {
			return err
		}
	}

	return nil
}

// LoadWeightsIntoNodes loads weights from a binary file into existing nodes
func LoadWeightsIntoNodes(path string, nodes []*gorgonia.Node) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return err
	}
	if string(magic) != "SKAT" {
		return fmt.Errorf("invalid magic number")
	}

	// Read version
	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return err
	}
	if version != 2 {
		return fmt.Errorf("unsupported version: %d (expected 2)", version)
	}

	weights := make(map[string][]float32)

	// Read each tensor
	for {
		// Read name length
		var nameLen uint32
		if err := binary.Read(f, binary.LittleEndian, &nameLen); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Read name
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(f, nameBytes); err != nil {
			return err
		}
		name := string(nameBytes)

		// Read shape
		var ndim uint32
		if err := binary.Read(f, binary.LittleEndian, &ndim); err != nil {
			return err
		}

		shape := make([]uint32, ndim)
		for i := range shape {
			if err := binary.Read(f, binary.LittleEndian, &shape[i]); err != nil {
				return err
			}
		}

		// Calculate total elements
		totalElems := uint32(1)
		for _, dim := range shape {
			totalElems *= dim
		}

		// Read data
		data := make([]float32, totalElems)
		if err := binary.Read(f, binary.LittleEndian, &data); err != nil {
			return err
		}

		weights[name] = data
	}

	// Set weights into the nodes
	for _, node := range nodes {
		name := node.Name()
		data, ok := weights[name]
		if !ok {
			return fmt.Errorf("missing weight: %s", name)
		}

		v := node.Value()
		if v == nil {
			return fmt.Errorf("node %s has no value", name)
		}

		// Copy data into the node's tensor
		dest := v.Data().([]float32)
		if len(dest) != len(data) {
			return fmt.Errorf("size mismatch for %s: expected %d, got %d", name, len(dest), len(data))
		}
		copy(dest, data)
	}

	return nil
}

// SaveWeightsToGCS saves network weights to Google Cloud Storage
func SaveWeightsToGCS(ctx context.Context, bucketName, objectName string, weights map[string]*gorgonia.Node) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)
	writer := obj.NewWriter(ctx)
	defer writer.Close()

	// Write magic number
	if _, err := writer.Write([]byte("SKAT")); err != nil {
		return err
	}

	// Write version
	version := uint32(2)
	if err := binary.Write(writer, binary.LittleEndian, version); err != nil {
		return err
	}

	// Write each tensor
	for name, node := range weights {
		// Write name length
		nameLen := uint32(len(name))
		if err := binary.Write(writer, binary.LittleEndian, nameLen); err != nil {
			return err
		}

		// Write name
		if _, err := writer.Write([]byte(name)); err != nil {
			return err
		}

		// Write shape
		shape := node.Shape()
		ndim := uint32(len(shape))
		if err := binary.Write(writer, binary.LittleEndian, ndim); err != nil {
			return err
		}

		for _, dim := range shape {
			if err := binary.Write(writer, binary.LittleEndian, uint32(dim)); err != nil {
				return err
			}
		}

		// Write data
		data := node.Value().Data().([]float32)
		if err := binary.Write(writer, binary.LittleEndian, data); err != nil {
			return err
		}
	}

	return writer.Close()
}

// LoadWeightsFromGCS loads weights from Google Cloud Storage into existing nodes
func LoadWeightsFromGCS(ctx context.Context, bucketName, objectName string, nodes []*gorgonia.Node) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS reader: %w", err)
	}
	defer reader.Close()

	// Read magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(reader, magic); err != nil {
		return err
	}
	if string(magic) != "SKAT" {
		return fmt.Errorf("invalid magic number")
	}

	// Read version
	var version uint32
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil {
		return err
	}
	if version != 2 {
		return fmt.Errorf("unsupported version: %d (expected 2)", version)
	}

	weights := make(map[string][]float32)

	// Read each tensor
	for {
		// Read name length
		var nameLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &nameLen); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Read name
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(reader, nameBytes); err != nil {
			return err
		}
		name := string(nameBytes)

		// Read shape
		var ndim uint32
		if err := binary.Read(reader, binary.LittleEndian, &ndim); err != nil {
			return err
		}

		shape := make([]int, ndim)
		totalSize := 1
		for i := 0; i < int(ndim); i++ {
			var dim uint32
			if err := binary.Read(reader, binary.LittleEndian, &dim); err != nil {
				return err
			}
			shape[i] = int(dim)
			totalSize *= int(dim)
		}

		// Read data
		data := make([]float32, totalSize)
		if err := binary.Read(reader, binary.LittleEndian, data); err != nil {
			return err
		}

		weights[name] = data
	}

	// Copy weights into nodes
	for _, node := range nodes {
		name := node.Name()
		data, ok := weights[name]
		if !ok {
			return fmt.Errorf("missing weight: %s", name)
		}

		v := node.Value()
		if v == nil {
			return fmt.Errorf("node %s has no value", name)
		}

		// Copy data into the node's tensor
		dest := v.Data().([]float32)
		if len(dest) != len(data) {
			return fmt.Errorf("size mismatch for %s: expected %d, got %d", name, len(dest), len(data))
		}
		copy(dest, data)
	}

	return nil
}
