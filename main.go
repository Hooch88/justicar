package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	supa "github.com/supabase-community/supabase-go"
	"google.golang.org/api/option"
)

// Player struct matches the 'player' table in Supabase.
type Player struct {
	ID            string           `json:"id,omitempty"`
	Name          string           `json:"name"`
	Alter         string           `json:"alter"`
	NetWorth      int64            `json:"net_worth"`
	StatusSummary string           `json:"status_summary"`
	History       []*genai.Content `json:"history"`
}

// Global variable for the Supabase client so all functions can access it.
var supaClient *supa.Client

func main() {
	// Load environment variables from our .env file.
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Connect to the Supabase database.
	supaClient, err = supa.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	if err != nil {
		log.Fatalf("Failed to connect to Supabase: %v", err)
	}
	fmt.Println("✅ Successfully connected to Supabase!")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Fatalf("Failed to create Generative client: %v", err)
	}
	defer client.Close()

	// Check if a player exists in the database.
	var players []Player
	_, err = supaClient.From("player").Select("*", "exact", false).Limit(1, "").ExecuteTo(&players)
	if err != nil {
		log.Fatalf("Error checking for player: %v", err)
	}

	var history []*genai.Content
	var chat *genai.ChatSession

	model := client.GenerativeModel("gemini-2.5-pro")
	chat = model.StartChat()

	if len(players) == 0 {
		fmt.Println("No player found in the database. Starting a new game...")
		history = startNewGame(ctx, client)
	} else {
		player := players[0]
		fmt.Printf("✅ Welcome back, %s! Loading your game...\n", player.Name)
		fmt.Println("--------------------------------------------------")
		fmt.Println(player.StatusSummary)
		fmt.Println("--------------------------------------------------")
		history = player.History
	}

	chat.History = history

	// Start the game loop.
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()

		resp, err := chat.SendMessage(ctx, genai.Text(input))
		if err != nil {
			log.Printf("Error sending message: %v", err)
			continue
		}

		fmt.Println("--------------------------------------------------")
		fmt.Println(getText(resp))
		fmt.Println("--------------------------------------------------")
	}
}

// startNewGame creates the initial records in the database and gets the opening scene.
func startNewGame(ctx context.Context, client *genai.Client) []*genai.Content {
	newPlayer := Player{
		Name:          "Caleb Reed",
		Alter:         "",
		NetWorth:      1000000000000, // 1 Trillion
		StatusSummary: "A 21-year-old student who just became the wealthiest person on Earth.",
	}

	var insertedPlayer []Player
	_, err := supaClient.From("player").Insert(newPlayer, false, "", "", "").ExecuteTo(&insertedPlayer)
	if err != nil {
		log.Fatalf("Failed to create new player: %v", err)
	}
	if len(insertedPlayer) == 0 {
		log.Fatal("Player was not created in the database, but no error was returned.")
	}
	fmt.Printf("New player '%s' saved to the database.\n", insertedPlayer[0].Name)

	fmt.Println("Generating opening scene...")
	model := client.GenerativeModel("gemini-2.5-pro")
	chat := model.StartChat()
	resp, err := chat.SendMessage(ctx, genai.Text(systemPrompt))
	if err != nil {
		log.Fatalf("Failed to generate opening scene: %v", err)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println(getText(resp))
	fmt.Println("--------------------------------------------------")

	return chat.History
}

func getText(resp *genai.GenerateContentResponse) string {
	var text string
	if resp != nil && len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				text += string(txt)
			}
		}
	}
	return text
}
