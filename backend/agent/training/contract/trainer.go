package contract

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"skat/agent/strategies"
	"skat/agent/strategies/encoding"
	"skat/game"

	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
)

type ContractExample struct {
	Features [encoding.ContractFeatureSize]float32
	Target   float32
	Weight   float32
}

type Trainer struct {
	model        *Model
	examples     []ContractExample
	batchSize    int
	learningRate float64
	l2Reg        float64
}

type Model struct {
	graph  *gorgonia.ExprGraph
	vm     gorgonia.VM
	solver gorgonia.Solver

	x      *gorgonia.Node
	target *gorgonia.Node
	weight *gorgonia.Node

	prediction *gorgonia.Node
	loss       *gorgonia.Node

	weights    []*gorgonia.Node
	weightsMap strategies.ContractNetworkWeights
}

func NewTrainer(batchSize int, learningRate, l2Reg float64) (*Trainer, error) {
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be positive")
	}
	trainer := &Trainer{batchSize: batchSize, learningRate: learningRate, l2Reg: l2Reg}
	model, err := trainer.createModel()
	if err != nil {
		return nil, err
	}
	trainer.model = model
	return trainer, nil
}

func (t *Trainer) createModel() (*Model, error) {
	g := gorgonia.NewGraph()
	weights := strategies.NewContractNetworkNodes(g)

	x := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, encoding.ContractFeatureSize),
		gorgonia.WithName("contract_features"))
	target := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 1),
		gorgonia.WithName("contract_target"))
	sampleWeight := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 1),
		gorgonia.WithName("contract_weight"))

	logits, err := weights.BuildContractNetwork(x, 0)
	if err != nil {
		return nil, err
	}
	prediction := gorgonia.Must(gorgonia.Sigmoid(logits))

	// Optimize the Bernoulli likelihood used by evaluation. Unlike squared
	// error through a sigmoid, cross-entropy keeps useful gradients for the
	// rare positive Schneider and Schwarz examples.
	one := gorgonia.NodeFromAny(g, float32(1), gorgonia.WithName("contract_one"))
	epsilon := gorgonia.NodeFromAny(g, float32(1e-7), gorgonia.WithName("contract_epsilon"))
	logPrediction := gorgonia.Must(gorgonia.Log(gorgonia.Must(gorgonia.Add(prediction, epsilon))))
	oneMinusPrediction := gorgonia.Must(gorgonia.Sub(one, prediction))
	logOneMinusPrediction := gorgonia.Must(gorgonia.Log(gorgonia.Must(gorgonia.Add(oneMinusPrediction, epsilon))))
	positive := gorgonia.Must(gorgonia.HadamardProd(target, logPrediction))
	oneMinusTarget := gorgonia.Must(gorgonia.Sub(one, target))
	negative := gorgonia.Must(gorgonia.HadamardProd(oneMinusTarget, logOneMinusPrediction))
	logLikelihood := gorgonia.Must(gorgonia.Add(positive, negative))
	weighted := gorgonia.Must(gorgonia.HadamardProd(logLikelihood, sampleWeight))
	loss := gorgonia.Must(gorgonia.Neg(gorgonia.Must(gorgonia.Mean(weighted))))

	trainable := weights.ToSlice()
	if t.l2Reg > 0 {
		l2Loss := gorgonia.NodeFromAny(g, float32(0.0), gorgonia.WithName("contract_l2_init"))
		for i := 0; i < len(trainable); i += 2 {
			sumSquared := gorgonia.Must(gorgonia.Sum(gorgonia.Must(gorgonia.Square(trainable[i]))))
			l2Loss = gorgonia.Must(gorgonia.Add(l2Loss, sumSquared))
		}
		regTerm := gorgonia.Must(gorgonia.Mul(l2Loss, gorgonia.NodeFromAny(g, float32(t.l2Reg), gorgonia.WithName("contract_l2_scale"))))
		loss = gorgonia.Must(gorgonia.Add(loss, regTerm))
	}

	if _, err := gorgonia.Grad(loss, trainable...); err != nil {
		return nil, err
	}

	vm := gorgonia.NewTapeMachine(g, gorgonia.BindDualValues(trainable...))
	solver := gorgonia.NewAdamSolver(gorgonia.WithLearnRate(t.learningRate))

	return &Model{
		graph:      g,
		vm:         vm,
		solver:     solver,
		x:          x,
		target:     target,
		weight:     sampleWeight,
		prediction: prediction,
		loss:       loss,
		weights:    trainable,
		weightsMap: weights,
	}, nil
}

func (t *Trainer) LoadDataset(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open dataset: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}
	headerIndex := make(map[string]int, len(header))
	for i, name := range header {
		headerIndex[strings.TrimSpace(strings.ToLower(name))] = i
	}

	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		ex, ok, err := parseExample(record, headerIndex)
		if err != nil {
			return err
		}
		if ok {
			t.examples = append(t.examples, ex)
		}
	}
	return nil
}

func parseExample(record []string, header map[string]int) (ContractExample, bool, error) {
	targetIdx, ok := findColumn(header, "target", "win", "win_probability", "declarer_won")
	if !ok || targetIdx >= len(record) {
		return ContractExample{}, false, nil
	}
	target, err := strconv.ParseFloat(strings.TrimSpace(record[targetIdx]), 32)
	if err != nil {
		return ContractExample{}, false, fmt.Errorf("invalid target %q: %w", record[targetIdx], err)
	}
	ex := ContractExample{Target: float32(clamp01Float(target)), Weight: 1}
	if weightIdx, ok := findColumn(header, "weight", "sample_weight"); ok && weightIdx < len(record) {
		if weight, err := strconv.ParseFloat(strings.TrimSpace(record[weightIdx]), 32); err == nil && weight > 0 {
			ex.Weight = float32(weight)
		}
	}

	if hasFeatureColumns(header) {
		for i := 0; i < encoding.ContractFeatureSize; i++ {
			idx := header[fmt.Sprintf("f%d", i)]
			if idx >= len(record) {
				return ContractExample{}, false, fmt.Errorf("missing f%d value", i)
			}
			value, err := strconv.ParseFloat(strings.TrimSpace(record[idx]), 32)
			if err != nil {
				return ContractExample{}, false, fmt.Errorf("invalid f%d value %q: %w", i, record[idx], err)
			}
			ex.Features[i] = float32(value)
		}
		return ex, true, nil
	}

	handIdx, handOK := findColumn(header, "hand", "cards")
	modeIdx, modeOK := findColumn(header, "mode", "game_mode")
	if !handOK || !modeOK || handIdx >= len(record) || modeIdx >= len(record) {
		return ContractExample{}, false, nil
	}
	hand, err := game.ParseCards(strings.TrimSpace(record[handIdx]))
	if err != nil {
		return ContractExample{}, false, fmt.Errorf("invalid hand %q: %w", record[handIdx], err)
	}
	mode := game.GameMode(strings.TrimSpace(strings.ToLower(record[modeIdx])))
	suit := game.NoSuit
	if suitIdx, ok := findColumn(header, "suit", "trump_suit", "trump"); ok && suitIdx < len(record) {
		suitValue := strings.TrimSpace(record[suitIdx])
		if suitValue != "" {
			parsedSuit, err := game.ParseSuit(suitValue)
			if err != nil {
				return ContractExample{}, false, err
			}
			suit = parsedSuit
		}
	}
	playedHand := readBoolColumn(header, record, "played_hand")
	announcedSchneider := readBoolColumn(header, record, "announced_schneider")
	announcedSchwarz := readBoolColumn(header, record, "announced_schwarz")
	encoded := encoding.EncodeNeuralContract(hand, mode, suit, playedHand, announcedSchneider, announcedSchwarz)
	ex.Features = encoded.ToSlice()
	return ex, true, nil
}

func readBoolColumn(header map[string]int, record []string, name string) bool {
	idx, ok := header[name]
	if !ok || idx >= len(record) {
		return false
	}
	value := strings.TrimSpace(strings.ToLower(record[idx]))
	return value == "1" || value == "true" || value == "yes"
}

func hasFeatureColumns(header map[string]int) bool {
	for i := 0; i < encoding.ContractFeatureSize; i++ {
		if _, ok := header[fmt.Sprintf("f%d", i)]; !ok {
			return false
		}
	}
	return true
}

func findColumn(header map[string]int, names ...string) (int, bool) {
	for _, name := range names {
		if idx, ok := header[name]; ok {
			return idx, true
		}
	}
	return 0, false
}

func (t *Trainer) Train() (loss, rmse float64, err error) {
	rand.Shuffle(len(t.examples), func(i, j int) {
		t.examples[i], t.examples[j] = t.examples[j], t.examples[i]
	})

	batches := len(t.examples) / t.batchSize
	if batches == 0 {
		return 0, 0, nil
	}
	for i := 0; i < batches; i++ {
		batch := t.examples[i*t.batchSize : (i+1)*t.batchSize]
		batchLoss, batchRMSE, err := t.trainBatch(batch)
		if err != nil {
			return 0, 0, err
		}
		loss += batchLoss
		rmse += batchRMSE
	}
	return loss / float64(batches), rmse / float64(batches), nil
}

func (t *Trainer) trainBatch(batch []ContractExample) (loss, rmse float64, err error) {
	featureData := make([]float32, len(batch)*encoding.ContractFeatureSize)
	targetData := make([]float32, len(batch))
	weightData := make([]float32, len(batch))
	for i, ex := range batch {
		copy(featureData[i*encoding.ContractFeatureSize:(i+1)*encoding.ContractFeatureSize], ex.Features[:])
		targetData[i] = ex.Target
		weightData[i] = ex.Weight
		if weightData[i] <= 0 {
			weightData[i] = 1
		}
	}

	gorgonia.Let(t.model.x, tensor.New(tensor.WithBacking(featureData), tensor.WithShape(len(batch), encoding.ContractFeatureSize)))
	gorgonia.Let(t.model.target, tensor.New(tensor.WithBacking(targetData), tensor.WithShape(len(batch), 1)))
	gorgonia.Let(t.model.weight, tensor.New(tensor.WithBacking(weightData), tensor.WithShape(len(batch), 1)))

	if err := t.model.vm.RunAll(); err != nil {
		return 0, 0, err
	}

	lossVal, err := scalarFloat32(t.model.loss.Value().Data())
	if err != nil {
		return 0, 0, err
	}

	predictions := t.model.prediction.Value().Data().([]float32)
	var squaredError float64
	for i := range batch {
		diff := float64(predictions[i] - targetData[i])
		squaredError += diff * diff
	}
	rmse = math.Sqrt(squaredError / float64(len(batch)))

	valueGrads := make([]gorgonia.ValueGrad, 0, len(t.model.weights))
	for _, w := range t.model.weights {
		valueGrads = append(valueGrads, w)
	}
	if err := t.model.solver.Step(valueGrads); err != nil {
		return 0, 0, err
	}
	t.model.vm.Reset()
	return float64(lossVal), rmse, nil
}

func scalarFloat32(data any) (float32, error) {
	switch v := data.(type) {
	case float32:
		return v, nil
	case []float32:
		if len(v) == 0 {
			return 0, fmt.Errorf("empty scalar slice")
		}
		return v[0], nil
	default:
		return 0, fmt.Errorf("unexpected scalar type: %T", data)
	}
}

func clamp01Float(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func (t *Trainer) GetWeights() strategies.ContractNetworkWeights {
	return t.model.weightsMap
}

func (t *Trainer) SetWeights(weights strategies.ContractNetworkWeights) error {
	return t.model.weightsMap.CopyFrom(weights)
}

func (t *Trainer) GetDatasetSize() int {
	return len(t.examples)
}
