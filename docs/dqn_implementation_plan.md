# Deep Q-Learning for Skat Cardplay - Implementation Plan

## Why DQN over Current Approach?

### Current Issues
- Supervised learning plateaus at 32% win rate
- Exploration (random moves) degrades performance to 13.6%
- Game-level rewards too coarse for credit assignment
- Imitating weak heuristic (27% win rate) limits ceiling

### DQN Advantages
- **Trick-level rewards** - immediate feedback per move
- **Smart exploration** - epsilon-greedy better than random
- **Continuous learning** - no separate collection/training phases
- **Experience replay** - learns efficiently from past games
- **Q-values** - directly estimate expected rewards

## State Encoding (162 features per network)

Each network (declarer and defender) receives a comprehensive encoding of the game state.
**Note:** PlayerRole feature removed - each network is specialized for its role.

### Card Presence (96 features)
- **MyHand [32]**: Binary vector of cards in my hand
- **TrickCards [32]**: Binary vector of cards in current trick
- **PlayedCards [32]**: Binary cumulative vector of all cards played so far

### Game Context (18 features) - PlayerRole removed for separate networks
- **GameMode [5]**: One-hot encoding [Grand, Clubs, Spades, Hearts, Diamonds]
- **TrickPosition [3]**: [leading, second, third] - affects valid plays
- **Scores [2]**: [declarer_score/120, opponent_score/120] - normalized
- **TricksRemaining [1]**: tricks_left / 10 - urgency signal
- **TrumpSituation [2]**: [my_trump_count/11, estimated_opponent_trumps/11]
- **Matadors [1]**: matadors / 4 - hand strength indicator
- **HandQuality [4]**: [strong_hand, winning_position, losing_position, critical_trick]

### Trick Analysis (16 features)
- **TrickValue [1]**: Current trick point value / 30 - how valuable to win
- **LeadSuitInHand [4]**: Count of lead suit cards per suit (normalized)
- **CanWinTrick [1]**: Binary flag if I can win this trick
- **HighestTrickRank [1]**: Highest rank in trick / 7
- **TrumpInTrick [1]**: Number of trump cards in current trick
- **PositionalFeatures [8]**: Advanced positional analysis (partner plays, etc.)

### Valid Moves Mask (32 features)
- **ValidMovesMask [32]**: Binary mask of legal cards to play
- Used to mask Q-values: `Q_masked = Q + (mask - 1) * 1e9`

## Architecture: Dueling DQN

### Why Dueling?
- Separates state value V(s) from action advantages A(s,a)
- More sample efficient for Skat (many forced/obvious plays)
- Learns faster when action values are similar

### Network Structure (Both Declarer and Defender)

Each role has its own identical architecture:

```
Input: [162] game state encoding (no PlayerRole feature)
   ↓
Shared: 162 → 256 → 256 → 128 (ReLU + Dropout)
   ↓
  / \
 /   \
Value Stream          Advantage Stream
128 → 64 → 1         128 → 64 → 32
    ↓                      ↓
   V(s)                  A(s,a)
    \                    /
     \                  /
      ↓                ↓
  Q(s,a) = V(s) + (A(s,a) - mean(A(s,a)))
           ↓
  Mask invalid actions: Q_valid = Q + (ValidMask - 1) * 1e9
           ↓
  Select action: argmax(Q_valid) or random (epsilon-greedy)
```

**Two instances of this architecture:**
- **declarerDQN** with **declarerTargetNet**
- **defenderDQN** with **defenderTargetNet**

Total: 4 networks (2 online + 2 target)

## Training Algorithm

### Experience Replay Buffers (Separate for Each Role)

```go
type Experience struct {
    State       [162]float32
    Action      int           // Card index (0-31)
    Reward      float32       // Trick reward
    NextState   [162]float32
    Done        bool          // Game ended
    ValidMask   [32]bool      // Valid cards at state
}

type ReplayBuffers struct {
    declarerBuffer  *CircularBuffer  // Experiences when playing as declarer
    defenderBuffer  *CircularBuffer  // Experiences when playing as defender
}
```

### Training Loop
```
1. Play game with epsilon-greedy policy
   - If declarer: use declarerDQN
   - If defender: use defenderDQN

2. Store all moves in appropriate replay buffer
   - Declarer moves → declarerBuffer
   - Defender moves → defenderBuffer

3. If buffers have enough samples (>1000):
   a. Sample batch from declarerBuffer
   b. Compute TD targets: reward + gamma * max Q_declarerTarget(s', a')
   c. Update declarerDQN to minimize (Q(s,a) - target)²

   d. Sample batch from defenderBuffer
   e. Compute TD targets: reward + gamma * max Q_defenderTarget(s', a')
   f. Update defenderDQN to minimize (Q(s,a) - target)²

4. Periodically copy online → target networks (both)
5. Decay epsilon gradually (shared across both roles)
```

## Key Hyperparameters

- **Replay buffer size**: 100,000 experiences (per buffer, so 200k total)
- **Batch size**: 256
- **Gamma (discount)**: 0.95 (future rewards matter)
- **Learning rate**: 0.0005 (lower than supervised)
- **Epsilon**: Start 1.0, decay to 0.1
- **Epsilon decay**: 0.995 per episode
- **Target network update**: Every 100 episodes
- **Training steps per game**: 4-8 batches

## Declarer vs Defender Strategies

The single DQN learns both roles through the **PlayerRole** encoding:

### Declarer Strategy (PlayerRole = [1, 0])
**Goal**: Maximize own trick points (need 61+ points to win)

**Learned behaviors:**
- Aggressive: win high-value tricks (10s, Aces)
- Trump management: use trumps efficiently to win tricks
- Count points: track progress toward 61 point threshold
- End-game: secure victory or minimize loss

**Reward signal:**
- Positive: win tricks with high point values
- Negative: lose 10s and Aces to opponents
- Final bonus: +1.0 if game won, -1.0 if lost

### Defender Strategy (PlayerRole = [0, 1])
**Goal**: Prevent declarer from reaching 61 points (team effort with partner)

**Learned behaviors:**
- Defensive: block declarer's high-value tricks
- Partnership: signal and coordinate with partner
- Trump depletion: force declarer to waste trumps
- Protection: save partner's 10s when possible

**Reward signal:**
- Positive: prevent declarer from winning high-value tricks
- Negative: let declarer capture 10s and Aces
- **Partnership coordination emerges from shared rewards**
- Final bonus: +1.0 if declarer fails, -1.0 if declarer wins

### Separate Networks for Each Role

**Why separate networks?**
- **Fundamentally different strategies**: Declarer plays alone, defenders play as team
- **Different objectives**: Declarer maximizes own points, defenders minimize declarer points
- **Conflicting signals**: Same card in same position means different things for each role
- **Better sample efficiency**: Each network sees only relevant examples
- **Clearer Q-values**: No need to disambiguate role through encoding

**Two DQN Networks:**

1. **Declarer Network**
   - Input: [162] features (remove PlayerRole, it's always declarer)
   - Learns: "How to win 61+ points playing solo"
   - Trained from: Games where DQN agent is declarer

2. **Defender Network**
   - Input: [162] features (remove PlayerRole, it's always defender)
   - Learns: "How to prevent declarer from 61 points as team"
   - Trained from: Games where DQN agent is defender
   - **Implicitly learns partnership** from shared rewards with partner

**Selection at runtime:**
```go
func (agent *DQNAgent) SelectCard(state *GameState, validMoves []Card) Card {
    if state.IsDeclarer(agent.Position) {
        return agent.declarerDQN.SelectAction(state, validMoves)
    } else {
        return agent.defenderDQN.SelectAction(state, validMoves)
    }
}
```

**Training separation:**
- Each game generates experiences for both networks
- Declarer moves → declarer replay buffer
- Defender moves (both defenders) → defender replay buffer
- Networks train independently on their respective buffers

**Data balance:**
- In each 3-player game: 1 declarer, 2 defenders
- Defender buffer fills ~2x faster than declarer buffer
- Both buffers naturally reach ~100k capacity over time
- This is fine! Defenders need to learn partnership coordination
- Declarer learns solo play (simpler, less data needed)

## Reward Structure

### Trick Reward (already computed in your code)
- Normalized to roughly [-1, 1]
- **Declarer**: Positive for winning high-value tricks, negative for losing them
- **Defender**: Positive for blocking declarer, negative for letting declarer score
- **This becomes the immediate reward in DQN**

### Terminal Reward (end of game)
- **If declarer**: +1.0 for win (≥61 points), -1.0 for loss
- **If defender**: +1.0 for win (declarer <61 points), -1.0 for loss
- Discounted back through all moves via Q-learning

**No separate game reward during training** - Q-learning bootstraps future value!

## Implementation Steps

1. **Create DQN network architecture** (Dueling variant)
2. **Experience replay buffer** with circular queue
3. **Training loop** with epsilon-greedy exploration
4. **TD target computation** with target network
5. **Evaluation harness** to track win rate progress
6. **Checkpointing** to save best models

## Expected Outcomes

- **Better exploration** - epsilon-greedy finds good moves faster
- **Move-level learning** - trick rewards provide immediate signal
- **No plateau from weak heuristic** - learns from own play
- **Target win rate**: 45-60% (vs heuristic baseline ~27%)

## Implementation Details

### Double DQN vs Dueling DQN

**Double DQN** addresses Q-value overestimation:
- Uses two networks to decorrelate action selection from evaluation
- `Target = reward + gamma * Q_target(s', argmax_a Q_online(s', a))`
- Less critical for Skat since we have immediate trick rewards

**Dueling DQN** separates state value from action advantages:
- `Q(s, a) = V(s) + A(s, a)`
- More sample efficient when many actions have similar value
- **Recommended for Skat**

### Partnership Coordination
- State encoding includes "cards played by partner" and "current trick"
- Q-network implicitly learns: "if partner played high card, play low card"
- Emerges from maximizing trick rewards across many games
- No explicit multi-agent algorithm needed

### Comparison: Supervised vs DQN

| Current (Supervised) | DQN |
|---------------------|-----|
| Collect 50k examples | Play continuously |
| Train 20 epochs | Train every move |
| Discard old data | Keep in replay buffer |
| Random exploration (15%) | Epsilon-greedy exploration |
| Game-level rewards | Trick-level rewards |
| Fixed policy during collection | Improving policy during training |

## Next Steps

1. Design DQN architecture in Go
2. Implement experience replay buffer
3. Create training loop with epsilon-greedy
4. Test on small scale (1000 games)
5. Full training run and evaluation

---

**Key Insight:** Use trick rewards as immediate feedback rather than waiting for game outcomes. This solves the credit assignment problem that plagued the supervised learning approach.
