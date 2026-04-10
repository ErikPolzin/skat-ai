package agent

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/storage"
)

// SaveQTable saves the Q-table to a JSON file
func (ba *BiddingAgent) SaveQTable(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	// Convert map[int]map[int]float64 to JSON-friendly format
	data := struct {
		QTable  map[int]map[int]float64 `json:"q_table"`
		Epsilon float64                 `json:"epsilon"`
	}{
		QTable:  ba.qTable,
		Epsilon: ba.Epsilon,
	}

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode Q-table: %w", err)
	}

	return nil
}

// LoadQTable loads the Q-table from a JSON file
func (ba *BiddingAgent) LoadQTable(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var data struct {
		QTable  map[int]map[int]float64 `json:"q_table"`
		Epsilon float64                 `json:"epsilon"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode Q-table: %w", err)
	}

	ba.qTable = data.QTable
	ba.Epsilon = data.Epsilon

	return nil
}

// SaveQTableBinary saves the Q-table in binary format using gob encoding
func (ba *BiddingAgent) SaveQTableBinary(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	return ba.encodeQTableGob(file)
}

// LoadQTableBinary loads the Q-table from a binary gob file
func (ba *BiddingAgent) LoadQTableBinary(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return ba.decodeQTableGob(file)
}

// SaveQTableToGCS saves the Q-table to Google Cloud Storage
// bucketName: e.g. "my-skat-models"
// objectName: e.g. "qtables/bidding_v1.gob"
func (ba *BiddingAgent) SaveQTableToGCS(ctx context.Context, bucketName, objectName string, binary bool) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)
	writer := obj.NewWriter(ctx)

	var encodeErr error
	if binary {
		encodeErr = ba.encodeQTableGob(writer)
	} else {
		encoder := json.NewEncoder(writer)
		data := struct {
			QTable  map[int]map[int]float64 `json:"q_table"`
			Epsilon float64                 `json:"epsilon"`
			Alpha   float64                 `json:"alpha"`
			Gamma   float64                 `json:"gamma"`
		}{
			QTable:  ba.qTable,
			Epsilon: ba.Epsilon,
			Alpha:   ba.alpha,
			Gamma:   ba.gamma,
		}
		encodeErr = encoder.Encode(data)
	}

	if encodeErr != nil {
		writer.Close()
		return fmt.Errorf("failed to encode Q-table: %w", encodeErr)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	return nil
}

// LoadQTableFromGCS loads the Q-table from Google Cloud Storage
func (ba *BiddingAgent) LoadQTableFromGCS(ctx context.Context, bucketName, objectName string, binary bool) error {
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

	if binary {
		return ba.decodeQTableGob(reader)
	}

	var data struct {
		QTable  map[int]map[int]float64 `json:"q_table"`
		Epsilon float64                 `json:"epsilon"`
		Alpha   float64                 `json:"alpha"`
		Gamma   float64                 `json:"gamma"`
	}

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode Q-table: %w", err)
	}

	ba.qTable = data.QTable
	ba.Epsilon = data.Epsilon
	if data.Alpha > 0 {
		ba.alpha = data.Alpha
	}
	if data.Gamma > 0 {
		ba.gamma = data.Gamma
	}

	return nil
}

// encodeQTableGob encodes the Q-table using gob format to the writer
func (ba *BiddingAgent) encodeQTableGob(w io.Writer) error {
	data := struct {
		QTable  map[int]map[int]float64
		Epsilon float64
		Alpha   float64
		Gamma   float64
	}{
		QTable:  ba.qTable,
		Epsilon: ba.Epsilon,
		Alpha:   ba.alpha,
		Gamma:   ba.gamma,
	}

	encoder := gob.NewEncoder(w)
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to gob encode: %w", err)
	}
	return nil
}

// decodeQTableGob decodes the Q-table from gob format
func (ba *BiddingAgent) decodeQTableGob(r io.Reader) error {
	var data struct {
		QTable  map[int]map[int]float64
		Epsilon float64
		Alpha   float64
		Gamma   float64
	}

	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to gob decode: %w", err)
	}

	ba.qTable = data.QTable
	ba.Epsilon = data.Epsilon
	ba.alpha = data.Alpha
	ba.gamma = data.Gamma

	return nil
}

// GetQTableStats returns statistics about the Q-table
func (ba *BiddingAgent) GetQTableStats() map[string]interface{} {
	totalStates := len(ba.qTable)
	totalStateActions := 0

	minQ := 999999.0
	maxQ := -999999.0
	sumQ := 0.0
	count := 0

	for _, actions := range ba.qTable {
		totalStateActions += len(actions)
		for _, q := range actions {
			if q < minQ {
				minQ = q
			}
			if q > maxQ {
				maxQ = q
			}
			sumQ += q
			count++
		}
	}

	avgQ := 0.0
	if count > 0 {
		avgQ = sumQ / float64(count)
	}

	return map[string]interface{}{
		"total_states":        totalStates,
		"total_state_actions": totalStateActions,
		"min_q":              minQ,
		"max_q":              maxQ,
		"avg_q":              avgQ,
		"epsilon":            ba.Epsilon,
	}
}
