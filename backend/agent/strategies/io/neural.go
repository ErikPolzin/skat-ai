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

// SaveCombinedCardPlayWeights saves both declarer and defender weights to a single file
func SaveCombinedCardPlayWeights(path string, declarerWeights, defenderWeights map[string]*gorgonia.Node) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write magic number
	if _, err := f.Write([]byte("SKAT")); err != nil {
		return err
	}

	// Write version (3 for combined format)
	version := uint32(3)
	if err := binary.Write(f, binary.LittleEndian, version); err != nil {
		return err
	}

	// Helper to write weights with role prefix
	writeWeights := func(weights map[string]*gorgonia.Node, prefix string) error {
		for name, node := range weights {
			// Prefix name with role
			fullName := prefix + name

			// Write name length
			nameLen := uint32(len(fullName))
			if err := binary.Write(f, binary.LittleEndian, nameLen); err != nil {
				return err
			}

			// Write name
			if _, err := f.Write([]byte(fullName)); err != nil {
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

	// Write declarer weights
	if err := writeWeights(declarerWeights, "declarer_"); err != nil {
		return fmt.Errorf("failed to write declarer weights: %w", err)
	}

	// Write defender weights
	if err := writeWeights(defenderWeights, "defender_"); err != nil {
		return fmt.Errorf("failed to write defender weights: %w", err)
	}

	return nil
}

// LoadCombinedCardPlayWeights loads both declarer and defender weights from a single file (local or GCS)
// Supports both local paths and GCS URIs (gs://bucket/path)
func LoadCombinedCardPlayWeights(path string, declarerGraph, defenderGraph *gorgonia.ExprGraph) (declarerWeights, defenderWeights map[string]*gorgonia.Node, err error) {
	// Check if GCS path
	if len(path) > 5 && path[:5] == "gs://" {
		// Parse GCS path
		remainder := path[5:]
		slashIdx := -1
		for i, ch := range remainder {
			if ch == '/' {
				slashIdx = i
				break
			}
		}
		if slashIdx == -1 {
			return nil, nil, fmt.Errorf("invalid GCS path: %s", path)
		}
		bucket := remainder[:slashIdx]
		objectPath := remainder[slashIdx+1:]

		return loadCombinedCardPlayWeightsFromGCS(context.Background(), bucket, objectPath, declarerGraph, defenderGraph)
	}

	// Local file
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	return loadCombinedCardPlayWeightsFromReader(f, declarerGraph, defenderGraph)
}

// loadCombinedCardPlayWeightsFromReader reads combined weights from an io.Reader
func loadCombinedCardPlayWeightsFromReader(reader io.Reader, declarerGraph, defenderGraph *gorgonia.ExprGraph) (declarerWeights, defenderWeights map[string]*gorgonia.Node, err error) {
	// Read magic number
	magic := make([]byte, 4)
	if _, err := io.ReadFull(reader, magic); err != nil {
		return nil, nil, err
	}
	if string(magic) != "SKAT" {
		return nil, nil, fmt.Errorf("invalid magic number")
	}

	// Read version
	var version uint32
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil {
		return nil, nil, err
	}
	if version != 3 {
		return nil, nil, fmt.Errorf("unsupported version: %d (expected 3 for combined format)", version)
	}

	declarerWeights = make(map[string]*gorgonia.Node)
	defenderWeights = make(map[string]*gorgonia.Node)

	// Read all weights
	for {
		// Read name length
		var nameLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &nameLen); err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, err
		}

		// Read name
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(reader, nameBytes); err != nil {
			return nil, nil, err
		}
		fullName := string(nameBytes)

		// Read shape
		var ndim uint32
		if err := binary.Read(reader, binary.LittleEndian, &ndim); err != nil {
			return nil, nil, err
		}

		shape := make([]int, ndim)
		for i := range shape {
			var dim uint32
			if err := binary.Read(reader, binary.LittleEndian, &dim); err != nil {
				return nil, nil, err
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
		if err := binary.Read(reader, binary.LittleEndian, &data); err != nil {
			return nil, nil, err
		}

		// Determine role and strip prefix
		var g *gorgonia.ExprGraph
		var targetMap map[string]*gorgonia.Node
		var name string

		if len(fullName) > 9 && fullName[:9] == "declarer_" {
			g = declarerGraph
			targetMap = declarerWeights
			name = fullName[9:]
		} else if len(fullName) > 9 && fullName[:9] == "defender_" {
			g = defenderGraph
			targetMap = defenderWeights
			name = fullName[9:]
		} else {
			return nil, nil, fmt.Errorf("invalid weight name (missing role prefix): %s", fullName)
		}

		// Create tensor and node
		t := tensor.New(tensor.WithBacking(data), tensor.WithShape(shape...))
		node := gorgonia.NodeFromAny(g, t, gorgonia.WithName(name))
		targetMap[name] = node
	}

	return declarerWeights, defenderWeights, nil
}

// loadCombinedCardPlayWeightsFromGCS loads combined weights from GCS
func loadCombinedCardPlayWeightsFromGCS(ctx context.Context, bucketName, objectPath string, declarerGraph, defenderGraph *gorgonia.ExprGraph) (declarerWeights, defenderWeights map[string]*gorgonia.Node, err error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectPath)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCS reader: %w", err)
	}
	defer reader.Close()

	return loadCombinedCardPlayWeightsFromReader(reader, declarerGraph, defenderGraph)
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
