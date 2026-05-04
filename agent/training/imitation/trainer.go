package imitation

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"skat/agent/strategies"

	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
)

// ImitationExample represents a training example
type ImitationExample struct {
	State      [114]float32
	ValidMask  [32]float32
	Action     int
	IsDeclarer bool
}

// BehavioralCloningTrainer trains a network to imitate expert play using supervised learning
type BehavioralCloningTrainer struct {
	// Models (separate for declarer and defender)
	declarerNet *BehavioralCloningModel
	defenderNet *BehavioralCloningModel

	// Training data
	declarerExamples []ImitationExample
	defenderExamples []ImitationExample

	// Hyperparameters
	batchSize    int
	learningRate float64
	l2Reg        float64
}

// BehavioralCloningModel is the network for imitation learning
type BehavioralCloningModel struct {
	graph  *gorgonia.ExprGraph
	vm     gorgonia.VM
	solver gorgonia.Solver

	// Input nodes
	x          *gorgonia.Node // State features (114)
	validMask  *gorgonia.Node // Valid moves mask (32)
	targetMask *gorgonia.Node // One-hot target action (32)

	// Output nodes
	logits *gorgonia.Node // Action logits (32)
	probs  *gorgonia.Node // Action probabilities (softmax)
	loss   *gorgonia.Node // Cross-entropy loss

	// Learnable parameters
	weights    []*gorgonia.Node
	weightsMap strategies.CardPlayNetworkWeights
}

// NewBehavioralCloningTrainer creates a new behavioral cloning trainer
func NewBehavioralCloningTrainer(batchSize int, learningRate, l2Reg float64) (*BehavioralCloningTrainer, error) {
	trainer := &BehavioralCloningTrainer{
		batchSize:    batchSize,
		learningRate: learningRate,
		l2Reg:        l2Reg,
	}

	// Create declarer network
	declNet, err := trainer.createModel()
	if err != nil {
		return nil, fmt.Errorf("failed to create declarer network: %w", err)
	}
	trainer.declarerNet = declNet

	// Create defender network
	defNet, err := trainer.createModel()
	if err != nil {
		return nil, fmt.Errorf("failed to create defender network: %w", err)
	}
	trainer.defenderNet = defNet

	return trainer, nil
}

func (t *BehavioralCloningTrainer) createModel() (*BehavioralCloningModel, error) {
	g := gorgonia.NewGraph()

	// Create weights with random initialization
	weights := strategies.NewCardPlayNetworkNodes(g)

	// Input: batch_size x 114 (state)
	xState := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 114),
		gorgonia.WithName("bc_state"))

	// Valid moves mask: batch_size x 32
	validMask := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 32),
		gorgonia.WithName("bc_valid_mask"))

	// Target action (one-hot): batch_size x 32
	targetMask := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 32),
		gorgonia.WithName("bc_target"))

	// Concatenate state and valid mask
	x := gorgonia.Must(gorgonia.Concat(1, xState, validMask))

	// Build network (policy head only, no value head needed for behavioral cloning)
	policyLogits, _, err := weights.BuildCardPlayNetwork(x, 0) // No dropout
	if err != nil {
		return nil, err
	}

	// Mask invalid actions by setting their logits to very negative value
	// Use smaller value (-20) to avoid numerical instability in softmax
	maskMinusOne := gorgonia.Must(gorgonia.Sub(validMask, gorgonia.NewConstant(float32(1.0))))
	maskPenalty := gorgonia.Must(gorgonia.Mul(maskMinusOne, gorgonia.NewConstant(float32(20.0))))
	logitsMasked := gorgonia.Must(gorgonia.Add(policyLogits, maskPenalty))

	// Softmax to get probabilities
	probs := gorgonia.Must(gorgonia.SoftMax(logitsMasked))

	// Cross-entropy loss: -sum(target * log(probs + epsilon))
	// Add epsilon to prevent log(0) - use larger value for numerical stability
	epsilon := gorgonia.NewConstant(float32(1e-7))
	probsStable := gorgonia.Must(gorgonia.Add(probs, epsilon))
	logProbs := gorgonia.Must(gorgonia.Log(probsStable))
	crossEntropy := gorgonia.Must(gorgonia.HadamardProd(targetMask, logProbs))
	crossEntropy = gorgonia.Must(gorgonia.Sum(crossEntropy, 1))
	crossEntropy = gorgonia.Must(gorgonia.Neg(crossEntropy))
	loss := gorgonia.Must(gorgonia.Mean(crossEntropy))

	// Only compute gradients for policy weights (value head not used)
	allWeights := weights.ToSlice()
	policyWeights := allWeights[:10] // shared (6) + policy (4), exclude value (2)

	// Add L2 regularization
	if t.l2Reg > 0 {
		l2Loss := gorgonia.NodeFromAny(g, float32(0.0), gorgonia.WithName("l2_init"))
		for i := 0; i < len(policyWeights); i += 2 { // Only weights, not biases
			w := policyWeights[i]
			squared := gorgonia.Must(gorgonia.Square(w))
			sumSquared := gorgonia.Must(gorgonia.Sum(squared))
			l2Loss = gorgonia.Must(gorgonia.Add(l2Loss, sumSquared))
		}
		regTerm := gorgonia.Must(gorgonia.Mul(l2Loss, gorgonia.NodeFromAny(g, float32(t.l2Reg), gorgonia.WithName("l2_scale"))))
		loss = gorgonia.Must(gorgonia.Add(loss, regTerm))
	}

	// Compute gradients only for policy weights
	if _, err := gorgonia.Grad(loss, policyWeights...); err != nil {
		return nil, err
	}

	// Create VM and solver
	vm := gorgonia.NewTapeMachine(g, gorgonia.BindDualValues(policyWeights...))
	solver := gorgonia.NewAdamSolver(gorgonia.WithLearnRate(t.learningRate))

	// Create weights map
	weightsMap := make(strategies.CardPlayNetworkWeights)
	weightNames := []string{
		"shared.0.weight", "shared.0.bias",
		"shared.2.weight", "shared.2.bias",
		"shared.4.weight", "shared.4.bias",
		"policy.0.weight", "policy.0.bias",
		"policy.2.weight", "policy.2.bias",
		"value.0.weight", "value.0.bias",
	}
	for i, name := range weightNames {
		weightsMap[name] = allWeights[i]
	}

	return &BehavioralCloningModel{
		graph:      g,
		vm:         vm,
		solver:     solver,
		x:          xState,
		validMask:  validMask,
		targetMask: targetMask,
		logits:     logitsMasked,
		probs:      probs,
		loss:       loss,
		weights:    policyWeights,
		weightsMap: weightsMap,
	}, nil
}

// LoadDataset loads training examples from CSV file
func (t *BehavioralCloningTrainer) LoadDataset(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open dataset: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Skip header
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Read examples
	for {
		record, err := reader.Read()
		if err != nil {
			break // EOF or error
		}

		var ex ImitationExample

		// Parse state (114 features)
		for i := 0; i < 114; i++ {
			val, _ := strconv.ParseFloat(record[i], 32)
			ex.State[i] = float32(val)
		}

		// Parse valid mask (32 features)
		for i := 0; i < 32; i++ {
			val, _ := strconv.ParseFloat(record[114+i], 32)
			ex.ValidMask[i] = float32(val)
		}

		// Parse action
		ex.Action, _ = strconv.Atoi(record[146])

		// Parse role
		isDeclarer, _ := strconv.Atoi(record[147])
		ex.IsDeclarer = (isDeclarer == 1)

		// Add to appropriate dataset
		if ex.IsDeclarer {
			t.declarerExamples = append(t.declarerExamples, ex)
		} else {
			t.defenderExamples = append(t.defenderExamples, ex)
		}
	}

	return nil
}

// Train performs one epoch of training
func (t *BehavioralCloningTrainer) Train() (declarerLoss, defenderLoss float64, err error) {
	// Shuffle datasets
	rand.Shuffle(len(t.declarerExamples), func(i, j int) {
		t.declarerExamples[i], t.declarerExamples[j] = t.declarerExamples[j], t.declarerExamples[i]
	})
	rand.Shuffle(len(t.defenderExamples), func(i, j int) {
		t.defenderExamples[i], t.defenderExamples[j] = t.defenderExamples[j], t.defenderExamples[i]
	})

	// Train declarer network
	var declLoss float64
	declBatches := len(t.declarerExamples) / t.batchSize
	for i := 0; i < declBatches; i++ {
		batch := t.declarerExamples[i*t.batchSize : (i+1)*t.batchSize]
		loss, err := t.trainBatch(t.declarerNet, batch)
		if err != nil {
			return 0, 0, fmt.Errorf("declarer training error: %w", err)
		}
		declLoss += loss
	}
	if declBatches > 0 {
		declLoss /= float64(declBatches)
	}

	// Train defender network
	var defLoss float64
	defBatches := len(t.defenderExamples) / t.batchSize
	for i := 0; i < defBatches; i++ {
		batch := t.defenderExamples[i*t.batchSize : (i+1)*t.batchSize]
		loss, err := t.trainBatch(t.defenderNet, batch)
		if err != nil {
			return 0, 0, fmt.Errorf("defender training error: %w", err)
		}
		defLoss += loss
	}
	if defBatches > 0 {
		defLoss /= float64(defBatches)
	}

	return declLoss, defLoss, nil
}

func (t *BehavioralCloningTrainer) trainBatch(model *BehavioralCloningModel, batch []ImitationExample) (float64, error) {
	// Prepare batch data
	stateData := make([]float32, len(batch)*114)
	maskData := make([]float32, len(batch)*32)
	targetData := make([]float32, len(batch)*32)

	for i, ex := range batch {
		copy(stateData[i*114:(i+1)*114], ex.State[:])
		copy(maskData[i*32:(i+1)*32], ex.ValidMask[:])
		// One-hot encode the action
		targetData[i*32+ex.Action] = 1.0
	}

	// Create tensors
	stateTensor := tensor.New(tensor.WithBacking(stateData), tensor.WithShape(len(batch), 114))
	maskTensor := tensor.New(tensor.WithBacking(maskData), tensor.WithShape(len(batch), 32))
	targetTensor := tensor.New(tensor.WithBacking(targetData), tensor.WithShape(len(batch), 32))

	// Set inputs
	gorgonia.Let(model.x, stateTensor)
	gorgonia.Let(model.validMask, maskTensor)
	gorgonia.Let(model.targetMask, targetTensor)

	// Forward + backward
	if err := model.vm.RunAll(); err != nil {
		return 0, err
	}

	// Get loss
	lossData := model.loss.Value().Data()
	var lossVal float32
	switch v := lossData.(type) {
	case float32:
		lossVal = v
	case []float32:
		lossVal = v[0]
	default:
		return 0, fmt.Errorf("unexpected loss type: %T", lossData)
	}

	// Update weights
	var valueGrads []gorgonia.ValueGrad
	for _, w := range model.weights {
		valueGrads = append(valueGrads, w)
	}

	if err := model.solver.Step(valueGrads); err != nil {
		return 0, err
	}

	// Reset for next batch
	model.vm.Reset()

	return float64(lossVal), nil
}

// GetWeights returns trained network weights
func (t *BehavioralCloningTrainer) GetWeights() (declarerWeights, defenderWeights strategies.CardPlayNetworkWeights) {
	return t.declarerNet.weightsMap, t.defenderNet.weightsMap
}

// GetDatasetSizes returns number of examples for each role
func (t *BehavioralCloningTrainer) GetDatasetSizes() (int, int) {
	return len(t.declarerExamples), len(t.defenderExamples)
}
