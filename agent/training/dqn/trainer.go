package dqn

import (
	"fmt"
	"math"
	"math/rand"

	"skat/agent/strategies"
	"skat/agent/strategies/encoding"
	strategiesio "skat/agent/strategies/io"
	"skat/game"

	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
)

// DQNExperience represents a single experience tuple for Deep Q-Learning
type DQNExperience struct {
	State     [114]float32 // State encoding (114 features, simplified)
	Action    int          // Card index (0-31) that was taken
	Reward    float32      // Immediate trick reward
	NextState [114]float32 // Next state encoding
	Done      bool         // Whether game ended
	ValidMask [32]float32  // Valid moves at current state
	NextMask  [32]float32  // Valid moves at next state
}

// DQNReplayBuffer implements circular experience replay buffer
type DQNReplayBuffer struct {
	buffer   []DQNExperience
	capacity int
	index    int
	size     int
}

// NewDQNReplayBuffer creates a new replay buffer with given capacity
func NewDQNReplayBuffer(capacity int) *DQNReplayBuffer {
	return &DQNReplayBuffer{
		buffer:   make([]DQNExperience, capacity),
		capacity: capacity,
		index:    0,
		size:     0,
	}
}

// Add adds an experience to the buffer
func (rb *DQNReplayBuffer) Add(exp DQNExperience) {
	rb.buffer[rb.index] = exp
	rb.index = (rb.index + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

// Sample randomly samples a batch of experiences
func (rb *DQNReplayBuffer) Sample(batchSize int) []DQNExperience {
	if rb.size < batchSize {
		batchSize = rb.size
	}

	batch := make([]DQNExperience, batchSize)

	// Use reservoir sampling to avoid infinite loops
	// For small batches or when batchSize << size, simple rejection sampling is fine
	// For large batches, use Fisher-Yates shuffle on indices
	if batchSize > rb.size/2 {
		// Large batch: shuffle and take first batchSize elements
		indices := make([]int, rb.size)
		for i := 0; i < rb.size; i++ {
			indices[i] = i
		}
		// Fisher-Yates shuffle
		for i := rb.size - 1; i > 0; i-- {
			j := rand.Intn(i + 1)
			indices[i], indices[j] = indices[j], indices[i]
		}
		for i := 0; i < batchSize; i++ {
			batch[i] = rb.buffer[indices[i]]
		}
	} else {
		// Small batch: rejection sampling with safety limit
		indices := make(map[int]bool)
		for i := 0; i < batchSize; i++ {
			idx := rand.Intn(rb.size)
			attempts := 0
			for indices[idx] && attempts < rb.size*10 {
				idx = rand.Intn(rb.size)
				attempts++
			}
			if attempts >= rb.size*10 {
				// Safety: allow duplicates rather than hanging
				idx = rand.Intn(rb.size)
			}
			indices[idx] = true
			batch[i] = rb.buffer[idx]
		}
	}

	return batch
}

// Size returns current buffer size
func (rb *DQNReplayBuffer) Size() int {
	return rb.size
}

// DQNCardPlayModel encapsulates DQN network for card play
//
// Uses Dueling DQN architecture (Wang et al., 2016):
//   Q(s,a) = V(s) + (A(s,a) - mean_a(A(s,a)))
//
// Where:
//   - V(s) is the state value (how good is this game state)
//   - A(s,a) is the advantage of action a (how much better is action a vs average)
//   - The mean-centering makes the decomposition identifiable
//
// Benefits over standard DQN:
//   - Better gradient flow to shared layers
//   - Can learn state values even when action choice doesn't matter
//   - More stable learning in environments with many similar-valued actions
type DQNCardPlayModel struct {
	graph  *gorgonia.ExprGraph
	vm     gorgonia.VM
	solver gorgonia.Solver

	// Input nodes
	x          *gorgonia.Node // State features (146)
	validMask  *gorgonia.Node // Valid moves mask (32)
	targetQ    *gorgonia.Node // Target Q-values (32)
	actionMask *gorgonia.Node // One-hot action taken (32)

	// Output nodes
	qValues *gorgonia.Node // Q-values for all actions (32)
	loss    *gorgonia.Node // TD loss

	// Learnable parameters
	weights    []*gorgonia.Node
	weightsMap strategies.CardPlayNetworkWeights
}

// DQNCardPlayTrainer implements DQN training for card play with separate networks
type DQNCardPlayTrainer struct {
	// Models (separate online and target networks for each role)
	declarerOnlineNet *DQNCardPlayModel
	declarerTargetNet *DQNCardPlayModel
	defenderOnlineNet *DQNCardPlayModel
	defenderTargetNet *DQNCardPlayModel

	// Replay buffers (separate for declarer and defender)
	declarerBuffer *DQNReplayBuffer
	defenderBuffer *DQNReplayBuffer

	// Hyperparameters
	gamma        float32 // Discount factor
	learningRate float64 // Learning rate
	batchSize    int     // Batch size
	targetUpdate int     // Steps between target network updates
	l2Reg        float64 // L2 regularization

	// Training state
	stepCount    int     // Total training steps
	epsilon      float32 // Epsilon for epsilon-greedy
	epsilonDecay float32 // Epsilon decay rate
	epsilonMin   float32 // Minimum epsilon
}

// NewDQNCardPlayTrainer creates a new DQN trainer
func NewDQNCardPlayTrainer(
	bufferSize int,
	batchSize int,
	gamma float32,
	learningRate float64,
	l2Reg float64,
	epsilon float32,
	epsilonDecay float32,
	epsilonMin float32,
	strategy *strategies.DeepQLearningCardPlayStrategy,
) (*DQNCardPlayTrainer, error) {
	trainer := &DQNCardPlayTrainer{
		declarerBuffer: NewDQNReplayBuffer(bufferSize),
		defenderBuffer: NewDQNReplayBuffer(bufferSize),
		gamma:          gamma,
		learningRate:   learningRate,
		batchSize:      batchSize,
		targetUpdate:   1000, // Update target network every 1000 steps (was 100, too frequent)
		l2Reg:          l2Reg,
		stepCount:      0,
		epsilon:        epsilon,
		epsilonDecay:   epsilonDecay,
		epsilonMin:     epsilonMin,
	}

	// Create declarer online network
	declarerOnline, err := trainer.createDQNModel(strategy, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create declarer online network: %w", err)
	}
	trainer.declarerOnlineNet = declarerOnline

	// Create declarer target network
	declarerTarget, err := trainer.createDQNModel(strategy, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create declarer target network: %w", err)
	}
	trainer.declarerTargetNet = declarerTarget

	// Create defender online network
	defenderOnline, err := trainer.createDQNModel(strategy, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create defender online network: %w", err)
	}
	trainer.defenderOnlineNet = defenderOnline

	// Create defender target network
	defenderTarget, err := trainer.createDQNModel(strategy, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create defender target network: %w", err)
	}
	trainer.defenderTargetNet = defenderTarget

	// Copy weights from online to target networks
	if err := trainer.updateTargetNetworks(); err != nil {
		return nil, fmt.Errorf("failed to initialize target networks: %w", err)
	}

	return trainer, nil
}

func (t *DQNCardPlayTrainer) createDQNModel(strategy *strategies.DeepQLearningCardPlayStrategy, trainable bool) (*DQNCardPlayModel, error) {
	g := gorgonia.NewGraph()

	// Always create fresh weights - we have separate networks per role
	weights := strategies.NewCardPlayNetworkNodes(g)

	// Input: batch_size x 114 (state without valid mask)
	xState := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 114),
		gorgonia.WithName("dqn_state"))

	// Valid moves mask: batch_size x 32
	validMask := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 32),
		gorgonia.WithName("dqn_valid_mask"))

	// Target Q-values: batch_size x 32
	targetQ := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 32),
		gorgonia.WithName("dqn_target_q"))

	// Action mask (one-hot): batch_size x 32
	actionMask := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(t.batchSize, 32),
		gorgonia.WithName("dqn_action_mask"))

	// Concatenate state (114) and valid mask (32) to form full input (146) for network
	x := gorgonia.Must(gorgonia.Concat(1, xState, validMask))

	// Build Dueling DQN network
	// Architecture: Shared trunk splits into two heads:
	//   - Advantage head A(s,a): outputs 32 values (one per action)
	//   - Value head V(s): outputs 1 scalar (state value)
	// Final Q-values: Q(s,a) = V(s) + (A(s,a) - mean(A(s,a)))
	//
	// Note: The network is named "CardPlayNetwork" with "policy" and "value" heads
	// but for DQN we interpret them as advantage and value heads respectively.
	advantageLogits, valueLogits, err := weights.BuildCardPlayNetwork(x, 0) // No dropout for DQN
	if err != nil {
		return nil, err
	}

	// Dueling DQN aggregation: Q(s,a) = V(s) + (A(s,a) - mean(A(s,a)))
	// The mean-centering ensures identifiability: the advantage represents
	// relative action quality, not absolute value
	advantages := advantageLogits

	// State value from value head (already shape [batch_size, 1])
	stateValue := valueLogits

	// Mean advantage (across all actions)
	meanAdvantage := gorgonia.Must(gorgonia.Mean(advantages, 1)) // Shape: [batch_size]
	meanAdvantage = gorgonia.Must(gorgonia.Reshape(meanAdvantage, tensor.Shape{t.batchSize, 1}))

	// Use BroadcastAdd to add the mean and state value to all 32 actions
	// For centering: A(s,a) - mean(A)
	//   We need to subtract meanAdvantage from all 32 actions
	//   BroadcastSub will broadcast [batch_size, 1] to [batch_size, 32]
	centeredAdvantages := gorgonia.Must(gorgonia.BroadcastSub(advantages, meanAdvantage, nil, []byte{1}))

	// For Q-values: V(s) + (A(s,a) - mean(A))
	//   We need to add stateValue to all 32 actions
	qValues := gorgonia.Must(gorgonia.BroadcastAdd(centeredAdvantages, stateValue, nil, []byte{1}))

	// Apply valid moves mask: Q_masked = Q + (mask - 1) * 1e9
	// This makes invalid actions have very negative Q-values
	maskMinusOne := gorgonia.Must(gorgonia.Sub(validMask, gorgonia.NewConstant(float32(1.0))))
	maskPenalty := gorgonia.Must(gorgonia.Mul(maskMinusOne, gorgonia.NewConstant(float32(1e9))))
	qValuesMasked := gorgonia.Must(gorgonia.Add(qValues, maskPenalty))

	var loss *gorgonia.Node
	var vm gorgonia.VM
	var solver gorgonia.Solver

	if trainable {
		// TD Loss: MSE between predicted Q(s,a) and target Q-value
		// We only compute loss for the action that was taken (using actionMask)

		// Select Q-values for taken actions
		selectedQ := gorgonia.Must(gorgonia.HadamardProd(qValuesMasked, actionMask))
		selectedQ = gorgonia.Must(gorgonia.Sum(selectedQ, 1)) // Shape: [batch_size]

		// Select target Q-values for taken actions
		selectedTargetQ := gorgonia.Must(gorgonia.HadamardProd(targetQ, actionMask))
		selectedTargetQ = gorgonia.Must(gorgonia.Sum(selectedTargetQ, 1)) // Shape: [batch_size]

		// MSE loss
		diff := gorgonia.Must(gorgonia.Sub(selectedQ, selectedTargetQ))
		squared := gorgonia.Must(gorgonia.Square(diff))
		loss = gorgonia.Must(gorgonia.Mean(squared))

		// Add L2 regularization
		allWeights := weights.ToSlice()
		if t.l2Reg > 0 {
			l2Loss := gorgonia.NodeFromAny(g, float32(0.0), gorgonia.WithName("l2_init"))
			for i := 0; i < len(allWeights); i += 2 { // Only weights, not biases
				w := allWeights[i]
				squared := gorgonia.Must(gorgonia.Square(w))
				sumSquared := gorgonia.Must(gorgonia.Sum(squared))
				l2Loss = gorgonia.Must(gorgonia.Add(l2Loss, sumSquared))
			}
			regTerm := gorgonia.Must(gorgonia.Mul(l2Loss, gorgonia.NodeFromAny(g, float32(t.l2Reg), gorgonia.WithName("l2_scale"))))
			loss = gorgonia.Must(gorgonia.Add(loss, regTerm))
		}

		// Compute gradients
		if _, err := gorgonia.Grad(loss, allWeights...); err != nil {
			return nil, err
		}

		// Create VM and solver
		vm = gorgonia.NewTapeMachine(g, gorgonia.BindDualValues(allWeights...))
		solver = gorgonia.NewAdamSolver(gorgonia.WithLearnRate(t.learningRate))
	} else {
		// Target network: no gradients needed
		vm = gorgonia.NewTapeMachine(g)
	}

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
	allWeights := weights.ToSlice()
	for i, name := range weightNames {
		weightsMap[name] = allWeights[i]
	}

	return &DQNCardPlayModel{
		graph:      g,
		vm:         vm,
		solver:     solver,
		x:          xState,
		validMask:  validMask,
		targetQ:    targetQ,
		actionMask: actionMask,
		qValues:    qValuesMasked,
		loss:       loss,
		weights:    allWeights,
		weightsMap: weightsMap,
	}, nil
}

// StoreExperience adds an experience to the appropriate replay buffer
func (t *DQNCardPlayTrainer) StoreExperience(exp DQNExperience, isDeclarer bool) {
	if isDeclarer {
		t.declarerBuffer.Add(exp)
	} else {
		t.defenderBuffer.Add(exp)
	}
}

// Train performs one training step on both declarer and defender networks
func (t *DQNCardPlayTrainer) Train() (declarerLoss, defenderLoss float64, err error) {
	var dLoss, defLoss float64

	// Train declarer network if enough samples
	if t.declarerBuffer.Size() >= t.batchSize {
		batch := t.declarerBuffer.Sample(t.batchSize)
		loss, err := t.trainBatch(batch, true)
		if err != nil {
			return 0, 0, fmt.Errorf("declarer training error: %w", err)
		}
		dLoss = loss
	}

	// Train defender network if enough samples
	if t.defenderBuffer.Size() >= t.batchSize {
		batch := t.defenderBuffer.Sample(t.batchSize)
		loss, err := t.trainBatch(batch, false)
		if err != nil {
			return 0, 0, fmt.Errorf("defender training error: %w", err)
		}
		defLoss = loss
	}

	t.stepCount++

	// Update target networks periodically
	if t.stepCount%t.targetUpdate == 0 {
		if err := t.updateTargetNetworks(); err != nil {
			return 0, 0, fmt.Errorf("target update error: %w", err)
		}
	}

	return dLoss, defLoss, nil
}

func (t *DQNCardPlayTrainer) trainBatch(batch []DQNExperience, isDeclarer bool) (float64, error) {
	// Select the appropriate networks based on role
	var onlineNet, targetNet *DQNCardPlayModel
	if isDeclarer {
		onlineNet = t.declarerOnlineNet
		targetNet = t.declarerTargetNet
	} else {
		onlineNet = t.defenderOnlineNet
		targetNet = t.defenderTargetNet
	}

	batchSize := len(batch)

	// Prepare batch data
	stateData := make([]float32, batchSize*114)
	maskData := make([]float32, batchSize*32)
	actionMaskData := make([]float32, batchSize*32)
	nextStateData := make([]float32, batchSize*114)
	nextMaskData := make([]float32, batchSize*32)

	// Build batched inputs
	for i, exp := range batch {
		copy(stateData[i*114:(i+1)*114], exp.State[:])
		copy(maskData[i*32:(i+1)*32], exp.ValidMask[:])
		actionMaskData[i*32+exp.Action] = 1.0
		copy(nextStateData[i*114:(i+1)*114], exp.NextState[:])
		copy(nextMaskData[i*32:(i+1)*32], exp.NextMask[:])
	}

	// Get current Q-values for all states in one forward pass
	currentQBatch := t.getBatchQFromNetwork(onlineNet, stateData, maskData, batchSize)

	// Get next Q-values from target network in one forward pass
	nextQBatch := t.getBatchQFromNetwork(targetNet, nextStateData, nextMaskData, batchSize)

	// Compute target Q-values using Bellman equation
	targetQData := make([]float32, batchSize*32)
	for i, exp := range batch {
		// Copy current Q-values (no gradient for non-selected actions)
		copy(targetQData[i*32:(i+1)*32], currentQBatch[i*32:(i+1)*32])

		// Compute target Q-value for the action taken
		var targetQ float32
		if exp.Done {
			// Terminal state: Q = reward only
			targetQ = exp.Reward
		} else {
			// Non-terminal: Q = reward + gamma * max Q(s', a')
			// Find max Q among valid next actions
			maxNextQ := float32(-1e9)
			for j := 0; j < 32; j++ {
				if exp.NextMask[j] > 0 && nextQBatch[i*32+j] > maxNextQ {
					maxNextQ = nextQBatch[i*32+j]
				}
			}
			targetQ = exp.Reward + t.gamma*maxNextQ
		}

		// Update target for the action taken
		targetQData[i*32+exp.Action] = targetQ
	}

	// Create tensors
	stateTensor := tensor.New(tensor.WithBacking(stateData), tensor.WithShape(len(batch), 114))
	maskTensor := tensor.New(tensor.WithBacking(maskData), tensor.WithShape(len(batch), 32))
	actionMaskTensor := tensor.New(tensor.WithBacking(actionMaskData), tensor.WithShape(len(batch), 32))
	targetQTensor := tensor.New(tensor.WithBacking(targetQData), tensor.WithShape(len(batch), 32))

	// Set inputs
	gorgonia.Let(onlineNet.x, stateTensor)
	gorgonia.Let(onlineNet.validMask, maskTensor)
	gorgonia.Let(onlineNet.actionMask, actionMaskTensor)
	gorgonia.Let(onlineNet.targetQ, targetQTensor)

	// Forward + backward
	if err := onlineNet.vm.RunAll(); err != nil {
		return 0, err
	}

	// Get loss
	lossData := onlineNet.loss.Value().Data()
	var lossVal float32
	switch v := lossData.(type) {
	case float32:
		lossVal = v
	case []float32:
		lossVal = v[0]
	default:
		return 0, fmt.Errorf("unexpected loss type: %T", lossData)
	}

	// Update weights with gradient clipping
	var valueGrads []gorgonia.ValueGrad
	for _, w := range onlineNet.weights {
		valueGrads = append(valueGrads, w)
	}

	// Clip gradients to prevent exploding gradients in large network
	// Compute global gradient norm
	gradNorm := float32(0.0)
	for _, vg := range valueGrads {
		if grad, err := vg.Grad(); err == nil && grad != nil {
			gradData := grad.Data().([]float32)
			for _, g := range gradData {
				gradNorm += g * g
			}
		}
	}
	gradNorm = float32(math.Sqrt(float64(gradNorm)))

	// Clip if norm exceeds threshold
	clipThreshold := float32(10.0) // Max gradient norm
	if gradNorm > clipThreshold {
		scale := clipThreshold / gradNorm
		for _, vg := range valueGrads {
			if grad, err := vg.Grad(); err == nil && grad != nil {
				gradData := grad.Data().([]float32)
				for i := range gradData {
					gradData[i] *= scale
				}
			}
		}
	}

	if err := onlineNet.solver.Step(valueGrads); err != nil {
		return 0, err
	}

	// Reset for next batch
	onlineNet.vm.Reset()

	return float64(lossVal), nil
}

// getBatchQFromNetwork gets Q-values for a batch of states from a specific network
func (t *DQNCardPlayTrainer) getBatchQFromNetwork(net *DQNCardPlayModel, stateData, maskData []float32, batchSize int) []float32 {
	// Create tensors for batch
	stateTensor := tensor.New(tensor.WithBacking(stateData), tensor.WithShape(batchSize, 114))
	maskTensor := tensor.New(tensor.WithBacking(maskData), tensor.WithShape(batchSize, 32))
	dummyTarget := make([]float32, batchSize*32)
	dummyAction := make([]float32, batchSize*32)

	gorgonia.Let(net.x, stateTensor)
	gorgonia.Let(net.validMask, maskTensor)
	gorgonia.Let(net.targetQ, tensor.New(tensor.WithBacking(dummyTarget), tensor.WithShape(batchSize, 32)))
	gorgonia.Let(net.actionMask, tensor.New(tensor.WithBacking(dummyAction), tensor.WithShape(batchSize, 32)))

	// Forward pass
	net.vm.Reset()
	if err := net.vm.RunAll(); err != nil {
		return dummyTarget
	}

	// Get Q-values
	qData := net.qValues.Value().Data().([]float32)
	result := make([]float32, batchSize*32)
	copy(result, qData[:batchSize*32])

	return result
}

// updateTargetNetworks copies weights from online to target networks for both roles
func (t *DQNCardPlayTrainer) updateTargetNetworks() error {
	// Update declarer target network
	if err := t.declarerTargetNet.weightsMap.CopyFrom(t.declarerOnlineNet.weightsMap); err != nil {
		return fmt.Errorf("failed to update declarer target network: %w", err)
	}

	// Update defender target network
	if err := t.defenderTargetNet.weightsMap.CopyFrom(t.defenderOnlineNet.weightsMap); err != nil {
		return fmt.Errorf("failed to update defender target network: %w", err)
	}

	return nil
}

// SaveWeights saves both declarer and defender network weights separately
func (t *DQNCardPlayTrainer) SaveWeights(declarerPath, defenderPath string) error {
	if err := strategiesio.SaveWeights(declarerPath, t.declarerOnlineNet.weightsMap); err != nil {
		return fmt.Errorf("failed to save declarer weights: %w", err)
	}
	if err := strategiesio.SaveWeights(defenderPath, t.defenderOnlineNet.weightsMap); err != nil {
		return fmt.Errorf("failed to save defender weights: %w", err)
	}
	return nil
}

// DecayEpsilon reduces epsilon for epsilon-greedy exploration
func (t *DQNCardPlayTrainer) DecayEpsilon() {
	t.epsilon *= t.epsilonDecay
	if t.epsilon < t.epsilonMin {
		t.epsilon = t.epsilonMin
	}
}

// GetEpsilon returns current epsilon
func (t *DQNCardPlayTrainer) GetEpsilon() float32 {
	return t.epsilon
}

// SetEpsilon sets epsilon manually
func (t *DQNCardPlayTrainer) SetEpsilon(eps float32) {
	t.epsilon = eps
}

func (t *DQNCardPlayTrainer) SetEpsilonDecay(decay float32) {
	t.epsilonDecay = decay
}

func (t *DQNCardPlayTrainer) SetEpsilonMin(min float32) {
	t.epsilonMin = min
}

// GetBufferSizes returns sizes of declarer and defender buffers
func (t *DQNCardPlayTrainer) GetBufferSizes() (int, int) {
	return t.declarerBuffer.Size(), t.defenderBuffer.Size()
}

// GetOnlineWeights returns the current online network weights for creating agents
func (t *DQNCardPlayTrainer) GetOnlineWeights() (declarerWeights, defenderWeights strategies.CardPlayNetworkWeights) {
	return t.declarerOnlineNet.weightsMap, t.defenderOnlineNet.weightsMap
}

// EncodeStateToDQN converts game state to DQN state encoding
// Returns 114 active features + valid mask
func EncodeStateToDQN(gs *game.GameState, myPosition game.GamePosition, validMoves []game.Card) ([114]float32, [32]float32) {
	// Use the simplified DQN encoding
	dqnEnc := encoding.EncodeDQNCardPlay(gs, myPosition, validMoves)
	return dqnEnc.ToSlice(), dqnEnc.GetValidMask()
}
