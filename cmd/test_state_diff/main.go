package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

func main() {
	// Create a game
	gameReq := map[string]interface{}{}
	gameReqJSON, _ := json.Marshal(gameReq)
	resp, err := http.Post("http://localhost:8080/api/games", "application/json",
		bytes.NewReader(gameReqJSON))
	if err != nil {
		log.Fatal("Failed to create game:", err)
	}
	defer resp.Body.Close()

	var gameData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&gameData); err != nil {
		log.Fatal("Failed to decode game response:", err)
	}

	gameID := gameData["game_id"].(string)
	fmt.Printf("Created game: %s\n", gameID)

	// Connect WebSocket
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Failed to connect WebSocket:", err)
	}
	defer ws.Close()

	// Join the game
	playerID := "player-" + time.Now().Format("150405")
	joinMsg := Message{
		Type: "join_game",
		Data: map[string]interface{}{
			"game_id":     gameID,
			"player_id":   playerID,
			"player_name": "Test Player 1",
		},
	}
	if err := ws.WriteJSON(joinMsg); err != nil {
		log.Fatal("Failed to send join message:", err)
	}

	// Start listening for messages
	go func() {
		for {
			var msg Message
			if err := ws.ReadJSON(&msg); err != nil {
				log.Println("Read error:", err)
				return
			}

			// Check if it's a state update
			if msg.Type == "state_update" {
				diff, ok := msg.Data["diff"].(map[string]interface{})
				if ok {
					fmt.Printf("\n=== STATE DIFF RECEIVED ===\n")
					fmt.Printf("Type: %v\n", diff["type"])
					fmt.Printf("Timestamp: %v\n", diff["timestamp"])
					if playerID, ok := diff["player_id"]; ok && playerID != "" {
						fmt.Printf("Player ID: %v\n", playerID)
					}

					changes, ok := diff["changes"].(map[string]interface{})
					if ok {
						fmt.Println("Changes:")
						for k, v := range changes {
							fmt.Printf("  %s: %v\n", k, v)
						}
					}
					fmt.Printf("========================\n")
				}
			} else {
				fmt.Printf("\nReceived message type: %s\n", msg.Type)
				prettyData, _ := json.MarshalIndent(msg.Data, "", "  ")
				fmt.Println(string(prettyData))
			}
		}
	}()

	// Wait a bit for the join to process
	time.Sleep(2 * time.Second)

	// Add two agents to fill the game
	fmt.Println("\nAdding agents to the game...")

	for i := 2; i <= 3; i++ {
		addAgentMsg := Message{
			Type: "add_agent",
			Data: map[string]interface{}{
				"agent_type": "random",
				"agent_name": fmt.Sprintf("Agent %d", i-1),
			},
		}
		if err := ws.WriteJSON(addAgentMsg); err != nil {
			log.Printf("Failed to add agent %d: %v", i-1, err)
		}
		time.Sleep(1 * time.Second)
	}

	// Wait a bit then start the game
	time.Sleep(2 * time.Second)
	fmt.Println("\nStarting the game...")

	startMsg := Message{
		Type: "start_game",
		Data: map[string]interface{}{},
	}
	if err := ws.WriteJSON(startMsg); err != nil {
		log.Fatal("Failed to start game:", err)
	}

	// Keep the connection open to observe game state changes
	fmt.Println("\nObserving game state changes (press Ctrl+C to exit)...")
	select {}
}
