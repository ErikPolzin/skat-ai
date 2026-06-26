package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"skat/agent"
	"skat/agent/strategies"
	"skat/game"
)

type contractExample struct {
	Hand   game.Cards
	Mode   game.GameMode
	Suit   game.Suit
	Target bool
}

func main() {
	numExamples := flag.Int("examples", 100000, "Number of examples to collect for each contract type")
	outputFile := flag.String("output", ".data/contract_dataset.csv", "Output CSV file")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of parallel workers")
	seed := flag.Int64("seed", 1, "Random seed")
	flag.Parse()

	if *numExamples <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid examples value %d (expected > 0)\n", *numExamples)
		os.Exit(1)
	}
	if *workers <= 0 {
		*workers = 1
	}
	rand.Seed(*seed)

	fmt.Println("============================================================")
	fmt.Println("Skat Contract Dataset Generation")
	fmt.Println("============================================================")
	fmt.Printf("Examples per contract type: %d\n", *numExamples)
	fmt.Printf("Total examples: %d\n", *numExamples*len(contractModes))
	fmt.Printf("Output: %s\n", *outputFile)
	fmt.Printf("Deals: random hands with forced contracts\n")
	fmt.Printf("Play: heuristic discard and heuristic card play\n")
	fmt.Printf("Workers: %d\n\n", *workers)

	examplesChan := make(chan contractExample, *workers*4)
	stopChan := make(chan struct{})
	var wg sync.WaitGroup
	var needed atomic.Uint32
	needed.Store(needAllModes)

	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			agentConfig := newHeuristicAgentConfig()
			for {
				select {
				case <-stopChan:
					return
				default:
				}

				mode, ok := chooseNeededMode(needed.Load())
				if !ok {
					return
				}
				ex := playForcedContract(agentConfig, mode, randomTrumpSuit(mode))
				select {
				case examplesChan <- ex:
				case <-stopChan:
					return
				}
			}
		}()
	}

	byMode := make(map[game.GameMode][]contractExample)
	counts := make(map[string]int)
	for !bucketsComplete(byMode, *numExamples) {
		ex := <-examplesChan
		if len(byMode[ex.Mode]) < *numExamples {
			byMode[ex.Mode] = append(byMode[ex.Mode], ex)
			counts[bucketKey(ex)]++
			needed.Store(neededModeMask(byMode, *numExamples))
		}
		total := totalExamples(byMode)
		if total%1000 == 0 {
			printProgress(byMode, *numExamples, counts)
		}
	}

	close(stopChan)
	wg.Wait()

	dataset := flattenDataset(byMode)
	if err := saveDataset(*outputFile, dataset); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save dataset: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSaved %d examples to %s\n", len(dataset), *outputFile)
	printProgress(byMode, *numExamples, counts)
}

func newHeuristicAgentConfig() agent.AgentConfig {
	baseAgent := agent.NewAgentWithStrategies(
		"HeuristicContractData",
		strategies.NewHeuristicBiddingStrategy(),
		strategies.NewHeuristicGameChoiceStrategy(),
		agent.NewHeuristicCardPlayStrategy(),
	)
	return agent.NewThreeWayConfig(baseAgent, baseAgent.Clone(), baseAgent.Clone())
}

func playForcedContract(agentConfig agent.AgentConfig, mode game.GameMode, suit game.Suit) contractExample {
	g := agent.WithAgentPlayers(game.NewGame(), agentConfig).WithCardsDealt()

	for _, player := range g.Players {
		agent.GetAgentForPlayer(player).OnGameStart()
	}

	declarer := game.GamePosition(rand.Intn(3))
	g = g.WithDeclarer(declarer, 0)
	if _, err := g.SkatDecision(true); err != nil {
		panic(fmt.Sprintf("SkatDecision error: %v", err))
	}

	declarerAgent := agent.GetAgentForPlayer(g.Players[declarer])
	card1, card2 := declarerAgent.ChooseSkatDiscard(g.Players[declarer].Hand, mode, suit)
	if _, err := g.Discard(card1, card2); err != nil {
		panic(fmt.Sprintf("Discard error: %v", err))
	}
	if _, err := g.DeclareGame(mode, suit, false, false); err != nil {
		panic(fmt.Sprintf("DeclareGame error: %v", err))
	}

	hand := append(game.Cards(nil), g.Players[declarer].Hand...)
	agent.WithAgentCardPlay(g)
	return contractExample{
		Hand:   hand,
		Mode:   mode,
		Suit:   suit,
		Target: g.Result().DeclarerWon,
	}
}

func saveDataset(path string, dataset []contractExample) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"hand", "mode", "suit", "target"}); err != nil {
		return err
	}
	for _, ex := range dataset {
		hand := ex.Hand.String()
		target := "0"
		if ex.Target {
			target = "1"
		}
		if err := writer.Write([]string{
			hand,
			string(ex.Mode),
			ex.Suit.String(),
			target,
		}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func bucketKey(ex contractExample) string {
	outcome := "loss"
	if ex.Target {
		outcome = "win"
	}
	return string(ex.Mode) + "_" + outcome
}

var contractModes = []game.GameMode{game.ModeGrand, game.ModeSuit, game.ModeNull}

const (
	needGrand uint32 = 1 << iota
	needSuit
	needNull
	needAllModes = needGrand | needSuit | needNull
)

func chooseNeededMode(mask uint32) (game.GameMode, bool) {
	var modes []game.GameMode
	if mask&needGrand != 0 {
		modes = append(modes, game.ModeGrand)
	}
	if mask&needSuit != 0 {
		modes = append(modes, game.ModeSuit)
	}
	if mask&needNull != 0 {
		modes = append(modes, game.ModeNull)
	}
	if len(modes) == 0 {
		return "", false
	}
	return modes[rand.Intn(len(modes))], true
}

func randomTrumpSuit(mode game.GameMode) game.Suit {
	if mode != game.ModeSuit {
		return game.NoSuit
	}
	suits := []game.Suit{game.Clubs, game.Spades, game.Hearts, game.Diamonds}
	return suits[rand.Intn(len(suits))]
}

func bucketsComplete(byMode map[game.GameMode][]contractExample, target int) bool {
	for _, mode := range contractModes {
		if len(byMode[mode]) < target {
			return false
		}
	}
	return true
}

func neededModeMask(byMode map[game.GameMode][]contractExample, target int) uint32 {
	var mask uint32
	if len(byMode[game.ModeGrand]) < target {
		mask |= needGrand
	}
	if len(byMode[game.ModeSuit]) < target {
		mask |= needSuit
	}
	if len(byMode[game.ModeNull]) < target {
		mask |= needNull
	}
	return mask
}

func totalExamples(byMode map[game.GameMode][]contractExample) int {
	total := 0
	for _, examples := range byMode {
		total += len(examples)
	}
	return total
}

func flattenDataset(byMode map[game.GameMode][]contractExample) []contractExample {
	total := totalExamples(byMode)
	dataset := make([]contractExample, 0, total)
	for _, mode := range contractModes {
		dataset = append(dataset, byMode[mode]...)
	}
	return dataset
}

func printProgress(byMode map[game.GameMode][]contractExample, target int, counts map[string]int) {
	fmt.Printf(
		"  Grand %d/%d (%.1f%% win) | Suit %d/%d (%.1f%% win) | Null %d/%d (%.1f%% win)\n",
		len(byMode[game.ModeGrand]),
		target,
		winRate(counts, game.ModeGrand),
		len(byMode[game.ModeSuit]),
		target,
		winRate(counts, game.ModeSuit),
		len(byMode[game.ModeNull]),
		target,
		winRate(counts, game.ModeNull),
	)
}

func winRate(counts map[string]int, mode game.GameMode) float64 {
	wins := counts[string(mode)+"_win"]
	losses := counts[string(mode)+"_loss"]
	total := wins + losses
	if total == 0 {
		return 0
	}
	return float64(wins) * 100 / float64(total)
}
