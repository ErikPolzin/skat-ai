package strategies

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"

	"skat/agent/strategies/encoding"
	strategiesio "skat/agent/strategies/io"
	"skat/game"

	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
)

// CardPlayNetworkWeights holds Gorgonia nodes for card play network weights
type CardPlayNetworkWeights map[string]*gorgonia.Node

// NetworkInstance represents a single neural network for card play
type NetworkInstance struct {
	graph       *gorgonia.ExprGraph
	vm          gorgonia.VM
	input       *gorgonia.Node
	policy      *gorgonia.Node
	value       *gorgonia.Node
	weights     CardPlayNetworkWeights
	inferenceMu sync.Mutex
}

// NeuralCardPlayStrategy implements neural network inference for card play
// Uses separate networks for declarer and defender roles
// Can be trained via imitation learning or reinforcement learning (DQN)
type NeuralCardPlayStrategy struct {
	declarerNet *NetworkInstance
	defenderNet *NetworkInstance
	epsilon     float32
}

// GetName returns strategy name
func (s *NeuralCardPlayStrategy) GetName() string {
	return "Neural Card Play"
}

// NewNeuralCardPlayStrategy creates a new neural strategy with fresh initialized weights
func NewNeuralCardPlayStrategy() *NeuralCardPlayStrategy {
	return &NeuralCardPlayStrategy{
		declarerNet: createNetworkInstance(nil),
		defenderNet: createNetworkInstance(nil),
	}
}

// NewNeuralCardPlayStrategyFromWeights loads both declarer and defender networks from a single file
func NewNeuralCardPlayStrategyFromWeights(path string) (*NeuralCardPlayStrategy, error) {
	// Create graphs for loading weights
	declarerGraph := gorgonia.NewGraph()
	defenderGraph := gorgonia.NewGraph()

	// Load combined weights from single file
	declarerWeights, defenderWeights, err := strategiesio.LoadCombinedCardPlayWeights(path, declarerGraph, defenderGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to load weights: %w", err)
	}

	return &NeuralCardPlayStrategy{
		declarerNet: createNetworkInstance(declarerWeights),
		defenderNet: createNetworkInstance(defenderWeights),
	}, nil
}

// NewNeuralCardPlayStrategyFromWeightMaps creates a strategy from existing weight maps (for self-play)
func NewNeuralCardPlayStrategyFromWeightMaps(declarerWeights, defenderWeights CardPlayNetworkWeights) *NeuralCardPlayStrategy {
	return &NeuralCardPlayStrategy{
		declarerNet: createNetworkInstance(declarerWeights),
		defenderNet: createNetworkInstance(defenderWeights),
	}
}

// createNetworkInstance creates a network instance with optional pre-loaded weights
func createNetworkInstance(weights CardPlayNetworkWeights) *NetworkInstance {
	g := gorgonia.NewGraph()

	// Input node for single inference
	input := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(1, encoding.NetworkInputSize),
		gorgonia.WithName("input"))

	// Create or use provided weights
	if weights == nil {
		weights = NewCardPlayNetworkNodes(g)
	} else {
		weights = weights.Clone(g)
	}

	// Build forward pass graphs
	policy, value := buildCardPlayNetwork(input, weights)

	// Create VM for inference
	vm := gorgonia.NewTapeMachine(g)

	return &NetworkInstance{
		graph:   g,
		vm:      vm,
		input:   input,
		policy:  policy,
		value:   value,
		weights: weights,
	}
}

// SelectMove chooses a card using the appropriate network (declarer or defender)
func (s *NeuralCardPlayStrategy) SelectMove(gs *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	// Epsilon-greedy exploration
	if s.epsilon > 0 && rand.Float32() < s.epsilon {
		return validMoves[rand.Intn(len(validMoves))]
	}

	// Determine if current player is declarer
	isDeclarer := (gs.Declarer != nil && gs.CurrentPlayer == *gs.Declarer)

	// Select appropriate network
	var net *NetworkInstance
	if isDeclarer {
		net = s.declarerNet
	} else {
		net = s.defenderNet
	}

	// Get current player position
	myPosition := gs.CurrentPlayer

	// Encode game state plus valid-move mask.
	enc := encoding.EncodeNeuralCardPlay(gs, myPosition, validMoves)
	inputData := enc.ToNetworkInput()

	// Mutex required: Gorgonia's VM spawns internal goroutines that aren't thread-safe
	net.inferenceMu.Lock()
	defer net.inferenceMu.Unlock()

	// Set input tensor
	inputTensor := tensor.New(tensor.WithBacking(inputData[:]), tensor.WithShape(1, encoding.NetworkInputSize))
	gorgonia.Let(net.input, inputTensor)

	// Run forward pass
	net.vm.Reset()
	if err := net.vm.RunAll(); err != nil {
		// Ensure VM is reset even on error for next inference
		net.vm.Reset()
		// Log warning - this shouldn't happen during normal operation
		fmt.Fprintf(os.Stderr, "WARNING: DQN inference error: %v (falling back to first valid move)\n", err)
		// Fallback to first valid move on error
		return validMoves[0]
	}

	// Get Q-values (raw advantage values from policy head)
	qValues := net.policy.Value().Data().([]float32)

	// Select best valid move by Q-value (argmax over valid actions)
	bestCard := validMoves[0]
	bestQ := float32(-1e9)

	for _, card := range validMoves {
		cardIdx := encoding.CardToIndex(card)
		q := qValues[cardIdx]
		if q > bestQ {
			bestQ = q
			bestCard = card
		}
	}

	return bestCard
}

// SetExploration sets epsilon for exploration
func (s *NeuralCardPlayStrategy) SetExploration(epsilon float32) {
	s.epsilon = epsilon
}

// UpdateWeights replaces the network weights with new ones
func (s *NeuralCardPlayStrategy) UpdateWeights(declarerWeights, defenderWeights CardPlayNetworkWeights) {
	s.declarerNet = createNetworkInstance(declarerWeights)
	s.defenderNet = createNetworkInstance(defenderWeights)
}

// Clone creates a copy of the strategy
func (s *NeuralCardPlayStrategy) Clone() *NeuralCardPlayStrategy {
	return &NeuralCardPlayStrategy{
		declarerNet: createNetworkInstance(s.declarerNet.weights),
		defenderNet: createNetworkInstance(s.defenderNet.weights),
		epsilon:     s.epsilon,
	}
}

// OnTrickComplete is a no-op for DQN (state is encoded per-move)
func (s *NeuralCardPlayStrategy) OnTrickComplete(trick []game.Card) {
	// No state to update
}

// Reset is a no-op for DQN (stateless)
func (s *NeuralCardPlayStrategy) Reset() {
	// No state to reset
}

// ============================================================================
// NeuralCardPlayStrategy implementation
// ============================================================================

// initWeight creates and initializes a weight node with Xavier initialization
func initWeight(g *gorgonia.ExprGraph, shape tensor.Shape, name string) *gorgonia.Node {
	var node *gorgonia.Node
	if len(shape) == 2 {
		node = gorgonia.NewMatrix(g, tensor.Float32, gorgonia.WithShape(shape...), gorgonia.WithName(name))
	} else {
		node = gorgonia.NewVector(g, tensor.Float32, gorgonia.WithShape(shape...), gorgonia.WithName(name))
	}

	// Xavier initialization
	size := shape.TotalSize()
	data := make([]float32, size)
	var fanIn, fanOut int
	if len(shape) == 2 {
		fanIn = shape[1]
		fanOut = shape[0]
	} else {
		fanIn = size
		fanOut = size
	}
	stddev := math.Sqrt(2.0 / float64(fanIn+fanOut))
	for i := range data {
		data[i] = float32(rand.NormFloat64() * stddev)
	}
	gorgonia.Let(node, tensor.New(tensor.WithBacking(data), tensor.WithShape(shape...)))
	return node
}

// NewCardPlayNetworkNodes creates Gorgonia nodes for the card play network architecture
// and initializes them with Xavier initialization.
//
// Architecture (Dueling DQN style):
//
//	Input -> Shared trunk (384 -> 384 -> 256) -> Two heads:
//	  - Advantage head (256 -> 128 -> 32): Outputs advantage A(s,a) for each action
//	  - Value head (256 -> 1): Outputs state value V(s)
//
// Medium capacity (~270k params) - sweet spot between old 256 (157k) and large 512 (490k).
func NewCardPlayNetworkNodes(g *gorgonia.ExprGraph) CardPlayNetworkWeights {
	weights := make(CardPlayNetworkWeights)

	// Shared layers (input -> 384 -> 384 -> 256)
	weights["shared.0.weight"] = initWeight(g, tensor.Shape{384, encoding.NetworkInputSize}, "shared.0.weight")
	weights["shared.0.bias"] = initWeight(g, tensor.Shape{384}, "shared.0.bias")
	weights["shared.2.weight"] = initWeight(g, tensor.Shape{384, 384}, "shared.2.weight")
	weights["shared.2.bias"] = initWeight(g, tensor.Shape{384}, "shared.2.bias")
	weights["shared.4.weight"] = initWeight(g, tensor.Shape{256, 384}, "shared.4.weight")
	weights["shared.4.bias"] = initWeight(g, tensor.Shape{256}, "shared.4.bias")

	// Advantage head (256 -> 128 -> 32)
	// For DQN: outputs advantage A(s,a) for each of 32 possible cards
	weights["policy.0.weight"] = initWeight(g, tensor.Shape{128, 256}, "policy.0.weight")
	weights["policy.0.bias"] = initWeight(g, tensor.Shape{128}, "policy.0.bias")
	weights["policy.2.weight"] = initWeight(g, tensor.Shape{32, 128}, "policy.2.weight")
	weights["policy.2.bias"] = initWeight(g, tensor.Shape{32}, "policy.2.bias")

	// Value head (256 -> 1) - simplified, no hidden layer needed
	// For DQN: outputs state value V(s)
	weights["value.0.weight"] = initWeight(g, tensor.Shape{1, 256}, "value.0.weight")
	weights["value.0.bias"] = initWeight(g, tensor.Shape{1}, "value.0.bias")

	return weights
}

// linearLayer applies a linear transformation: y = xW^T + b
func linearLayer(x, weight, bias *gorgonia.Node) *gorgonia.Node {
	// Multiply input by transposed weight matrix
	y := gorgonia.Must(gorgonia.Mul(x, gorgonia.Must(gorgonia.Transpose(weight))))
	// Broadcast add bias
	return gorgonia.Must(gorgonia.BroadcastAdd(y, bias, nil, []byte{0}))
}

// reluActivation applies ReLU activation function
func reluActivation(x *gorgonia.Node) *gorgonia.Node {
	return gorgonia.Must(gorgonia.Rectify(x))
}

// softmaxActivation applies softmax activation function
func softmaxActivation(x *gorgonia.Node) *gorgonia.Node {
	return gorgonia.Must(gorgonia.SoftMax(x))
}

// buildCardPlayLogits builds card play network returning logits (for training)
func buildCardPlayLogits(x *gorgonia.Node, w CardPlayNetworkWeights, dropout float64) (*gorgonia.Node, *gorgonia.Node) {
	// Shared trunk (input -> 384 -> 384 -> 256)
	h1 := linearLayer(x, w["shared.0.weight"], w["shared.0.bias"])
	h1 = reluActivation(h1)
	if dropout > 0 {
		h1 = gorgonia.Must(gorgonia.Dropout(h1, dropout))
	}

	h2 := linearLayer(h1, w["shared.2.weight"], w["shared.2.bias"])
	h2 = reluActivation(h2)
	if dropout > 0 {
		h2 = gorgonia.Must(gorgonia.Dropout(h2, dropout))
	}

	h3 := linearLayer(h2, w["shared.4.weight"], w["shared.4.bias"])
	h3 = reluActivation(h3)
	if dropout > 0 {
		h3 = gorgonia.Must(gorgonia.Dropout(h3, dropout))
	}

	// Advantage head (256 -> 128 -> 32) - return logits
	p1 := linearLayer(h3, w["policy.0.weight"], w["policy.0.bias"])
	p1 = reluActivation(p1)
	if dropout > 0 {
		p1 = gorgonia.Must(gorgonia.Dropout(p1, dropout))
	}
	policyLogits := linearLayer(p1, w["policy.2.weight"], w["policy.2.bias"])

	// Value head (256 -> 1) - simplified, single layer
	valueLogits := linearLayer(h3, w["value.0.weight"], w["value.0.bias"])

	return policyLogits, valueLogits
}

// buildCardPlayNetwork constructs the forward pass for card play network (for inference)
func buildCardPlayNetwork(x *gorgonia.Node, w CardPlayNetworkWeights) (*gorgonia.Node, *gorgonia.Node) {
	policyLogits, valueLogits := buildCardPlayLogits(x, w, 0)

	// For DQN: return raw Q-values (advantage logits), not softmaxed probabilities
	// The Dueling DQN architecture combines these in the trainer, and inference
	// selects argmax over raw Q-values, not probabilities
	// NOTE: valueLogits are used in Dueling DQN combination in trainer
	return policyLogits, valueLogits
}

// ============================================================================
// CardPlayNetworkWeights methods
// ============================================================================

// Clone creates a new CardPlayNetworkWeights in a new graph with copied values
func (w CardPlayNetworkWeights) Clone(g *gorgonia.ExprGraph) CardPlayNetworkWeights {
	newWeights := NewCardPlayNetworkNodes(g)

	for name, srcNode := range w {
		dstNode := newWeights[name]
		data := srcNode.Value().Data().([]float32)

		// Deep copy the data
		dataCopy := make([]float32, len(data))
		copy(dataCopy, data)

		// Set value in new node
		t := tensor.New(tensor.WithBacking(dataCopy), tensor.WithShape(dstNode.Shape()...))
		gorgonia.Let(dstNode, t)
	}

	return newWeights
}

// CopyFrom copies weight values from another CardPlayNetworkWeights
func (w CardPlayNetworkWeights) CopyFrom(source CardPlayNetworkWeights) error {
	for name, dstNode := range w {
		srcNode, ok := source[name]
		if !ok {
			return fmt.Errorf("missing weight in source: %s", name)
		}

		data := srcNode.Value().Data().([]float32)

		// Deep copy the data
		dataCopy := make([]float32, len(data))
		copy(dataCopy, data)

		// Set value in destination node
		t := tensor.New(tensor.WithBacking(dataCopy), tensor.WithShape(dstNode.Shape()...))
		gorgonia.Let(dstNode, t)
	}

	return nil
}

// ToSlice returns all weight nodes as a slice in a consistent order
func (w CardPlayNetworkWeights) ToSlice() []*gorgonia.Node {
	return []*gorgonia.Node{
		// Shared layers
		w["shared.0.weight"], w["shared.0.bias"],
		w["shared.2.weight"], w["shared.2.bias"],
		w["shared.4.weight"], w["shared.4.bias"],
		// Advantage head
		w["policy.0.weight"], w["policy.0.bias"],
		w["policy.2.weight"], w["policy.2.bias"],
		// Value head (simplified, only 1 layer)
		w["value.0.weight"], w["value.0.bias"],
	}
}

// BuildCardPlayNetwork constructs the forward pass for card play (for trainer use - returns logits)
func (w CardPlayNetworkWeights) BuildCardPlayNetwork(x *gorgonia.Node, dropout float64) (*gorgonia.Node, *gorgonia.Node, error) {
	policyLogits, valueLogits := buildCardPlayLogits(x, w, dropout)
	return policyLogits, valueLogits, nil
}
