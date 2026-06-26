package strategies

import (
	"fmt"
	"os"

	"skat/agent/strategies/encoding"
	strategiesio "skat/agent/strategies/io"
	"skat/game"

	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
)

type ContractNetworkWeights map[string]*gorgonia.Node

type NeuralContractWinProbabilityEstimator struct {
	net *NetworkInstance
}

func NewNeuralContractWinProbabilityEstimator() *NeuralContractWinProbabilityEstimator {
	return &NeuralContractWinProbabilityEstimator{net: createContractNetworkInstance(nil)}
}

func NewNeuralContractWinProbabilityEstimatorFromWeights(path string) (*NeuralContractWinProbabilityEstimator, error) {
	g := gorgonia.NewGraph()
	weights := NewContractNetworkNodes(g)
	if err := strategiesio.LoadWeightsIntoNodes(path, weights.ToSlice()); err != nil {
		return nil, fmt.Errorf("failed to load contract weights: %w", err)
	}
	return &NeuralContractWinProbabilityEstimator{net: createContractNetworkInstance(weights)}, nil
}

func NewNeuralContractWinProbabilityEstimatorFromWeightMap(weights ContractNetworkWeights) *NeuralContractWinProbabilityEstimator {
	return &NeuralContractWinProbabilityEstimator{net: createContractNetworkInstance(weights)}
}

func (e *NeuralContractWinProbabilityEstimator) EstimateWinProbability(hand game.Cards, mode game.GameMode, suit game.Suit) float64 {
	enc := encoding.EncodeNeuralContract(hand, mode, suit)
	probability, err := e.EstimateEncodedWinProbability(enc.ToSlice())
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: contract inference error: %v (falling back to heuristic estimate)\n", err)
		return NewHeuristicContractWinProbabilityEstimator().EstimateWinProbability(hand, mode, suit)
	}
	return probability
}

func (e *NeuralContractWinProbabilityEstimator) EstimateEncodedWinProbability(inputData [encoding.ContractFeatureSize]float32) (float64, error) {
	e.net.inferenceMu.Lock()
	defer e.net.inferenceMu.Unlock()

	inputTensor := tensor.New(tensor.WithBacking(inputData[:]), tensor.WithShape(1, encoding.ContractFeatureSize))
	gorgonia.Let(e.net.input, inputTensor)

	e.net.vm.Reset()
	if err := e.net.vm.RunAll(); err != nil {
		e.net.vm.Reset()
		return 0, err
	}

	output := e.net.output.Value().Data().([]float32)
	if len(output) == 0 {
		return 0, fmt.Errorf("empty contract network output")
	}
	return float64(output[0]), nil
}

func (e *NeuralContractWinProbabilityEstimator) GetWeights() ContractNetworkWeights {
	return e.net.contractWeights
}

func createContractNetworkInstance(weights ContractNetworkWeights) *NetworkInstance {
	g := gorgonia.NewGraph()
	input := gorgonia.NewMatrix(g, tensor.Float32,
		gorgonia.WithShape(1, encoding.ContractFeatureSize),
		gorgonia.WithName("contract_input"))
	if weights == nil {
		weights = NewContractNetworkNodes(g)
	} else {
		weights = weights.Clone(g)
	}
	output := buildContractNetwork(input, weights)
	vm := gorgonia.NewTapeMachine(g)
	return &NetworkInstance{
		graph:           g,
		vm:              vm,
		input:           input,
		output:          output,
		contractWeights: weights,
	}
}

func NewContractNetworkNodes(g *gorgonia.ExprGraph) ContractNetworkWeights {
	weights := make(ContractNetworkWeights)
	weights["shared.0.weight"] = initWeight(g, tensor.Shape{128, encoding.ContractFeatureSize}, "shared.0.weight")
	weights["shared.0.bias"] = initWeight(g, tensor.Shape{128}, "shared.0.bias")
	weights["shared.2.weight"] = initWeight(g, tensor.Shape{64, 128}, "shared.2.weight")
	weights["shared.2.bias"] = initWeight(g, tensor.Shape{64}, "shared.2.bias")
	weights["value.0.weight"] = initWeight(g, tensor.Shape{1, 64}, "value.0.weight")
	weights["value.0.bias"] = initWeight(g, tensor.Shape{1}, "value.0.bias")
	return weights
}

func buildContractLogits(x *gorgonia.Node, w ContractNetworkWeights, dropout float64) *gorgonia.Node {
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

	return linearLayer(h2, w["value.0.weight"], w["value.0.bias"])
}

func buildContractNetwork(x *gorgonia.Node, w ContractNetworkWeights) *gorgonia.Node {
	return gorgonia.Must(gorgonia.Sigmoid(buildContractLogits(x, w, 0)))
}

func (w ContractNetworkWeights) Clone(g *gorgonia.ExprGraph) ContractNetworkWeights {
	newWeights := NewContractNetworkNodes(g)
	for name, srcNode := range w {
		dstNode := newWeights[name]
		data := srcNode.Value().Data().([]float32)
		dataCopy := make([]float32, len(data))
		copy(dataCopy, data)
		gorgonia.Let(dstNode, tensor.New(tensor.WithBacking(dataCopy), tensor.WithShape(dstNode.Shape()...)))
	}
	return newWeights
}

func (w ContractNetworkWeights) CopyFrom(source ContractNetworkWeights) error {
	for name, dstNode := range w {
		srcNode, ok := source[name]
		if !ok {
			return fmt.Errorf("missing weight in source: %s", name)
		}
		data := srcNode.Value().Data().([]float32)
		dataCopy := make([]float32, len(data))
		copy(dataCopy, data)
		gorgonia.Let(dstNode, tensor.New(tensor.WithBacking(dataCopy), tensor.WithShape(dstNode.Shape()...)))
	}
	return nil
}

func (w ContractNetworkWeights) ToSlice() []*gorgonia.Node {
	return []*gorgonia.Node{
		w["shared.0.weight"], w["shared.0.bias"],
		w["shared.2.weight"], w["shared.2.bias"],
		w["value.0.weight"], w["value.0.bias"],
	}
}

func (w ContractNetworkWeights) BuildContractNetwork(x *gorgonia.Node, dropout float64) (*gorgonia.Node, error) {
	return buildContractLogits(x, w, dropout), nil
}
