package agent

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"

	"cloud.google.com/go/storage"
)

// QTableData represents serialized Q-table data
type QTableData struct {
	QTable  map[int]map[int]float64 `json:"q_table"`
	Epsilon float64                 `json:"epsilon"`
}

// SaveQTableData saves Q-table data to a file
func SaveQTableData(data *QTableData, filename string, binary bool) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if binary {
		encoder := gob.NewEncoder(file)
		if err := encoder.Encode(data); err != nil {
			return fmt.Errorf("failed to gob encode: %w", err)
		}
	} else {
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(data); err != nil {
			return fmt.Errorf("failed to json encode: %w", err)
		}
	}

	return nil
}

// LoadQTableData loads Q-table data from a file
func LoadQTableData(filename string, binary bool) (*QTableData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var data QTableData

	if binary {
		decoder := gob.NewDecoder(file)
		if err := decoder.Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to gob decode: %w", err)
		}
	} else {
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to json decode: %w", err)
		}
	}

	return &data, nil
}

// SaveQTableDataToGCS saves Q-table data to Google Cloud Storage
func SaveQTableDataToGCS(ctx context.Context, data *QTableData, bucketName, objectName string, binary bool) error {
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
		encoder := gob.NewEncoder(writer)
		encodeErr = encoder.Encode(data)
	} else {
		encoder := json.NewEncoder(writer)
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

// LoadQTableDataFromGCS loads Q-table data from Google Cloud Storage
func LoadQTableDataFromGCS(ctx context.Context, bucketName, objectName string, binary bool) (*QTableData, error) {
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

	var data QTableData

	if binary {
		decoder := gob.NewDecoder(reader)
		if err := decoder.Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to gob decode: %w", err)
		}
	} else {
		decoder := json.NewDecoder(reader)
		if err := decoder.Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to json decode: %w", err)
		}
	}

	return &data, nil
}

// GetQTableStats returns statistics about a Q-table
func GetQTableStats(qTable map[int]map[int]float64, epsilon float64) map[string]interface{} {
	totalStates := len(qTable)
	totalStateActions := 0

	minQ := 999999.0
	maxQ := -999999.0
	sumQ := 0.0
	count := 0

	for _, actions := range qTable {
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
		"min_q":               minQ,
		"max_q":               maxQ,
		"avg_q":               avgQ,
		"epsilon":             epsilon,
	}
}
