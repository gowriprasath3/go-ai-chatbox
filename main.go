package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	openai "github.com/sashabaranov/go-openai"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type WSMessage struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

func main() {

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY env var is required")
	}
	client := openai.NewClient(apiKey)

	http.HandleFunc(" / ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(w, r, client)
	})

	fs := http.FileServer(http.Dir(". / web"))
	http.Handle(" / ", fs)
	addr := ":8080"
	log.Printf("Server running at %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleWS(w http.ResponseWriter, r *http.Request, client *openai.Client) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()
	ctx := context.Background()
	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Println("ReadJSON error:", err)
			return

		}
		if msg.Type == "user_message" {
			go handleUserMessage(ctx, conn, client, msg.Payload)
		}
	}
}

func handleUserMessage(ctx context.Context, conn *websocket.Conn, client *openai.Client, userText string) {
	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system",
				Content: "You are a helpful assistant."},
			{Role: "user",
				Content: userText},
		},
		Stream: true}
	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		sendError(conn, fmt.Sprintf("Stream error: %v", err))
		return
	}

	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				_ = conn.WriteJSON(WSMessage{Type: "ai_done", Payload: ""})
				return
			}
			sendError(conn, fmt.Sprintf("Recv error: %v", err))
			return
		}
		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta.Content
			if delta != "" {
				_ = conn.WriteJSON(WSMessage{Type: "ai_delta", Payload: delta})
			}
		}
	}
}

func sendError(conn *websocket.Conn, text string) {
	_ = conn.WriteJSON(WSMessage{Type: "error", Payload: text})
	time.Sleep(50 * time.Millisecond)
}
