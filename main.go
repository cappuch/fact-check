package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	if err := InitDB(); err != nil {
		fmt.Println("Error initializing database:", err)
		return
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/factcheck", handleFactCheck)

	go func() {
		fmt.Println("Web server running on http://localhost:8080")
		http.ListenAndServe(":8080", nil)
	}()

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		fmt.Println("Error: DISCORD_TOKEN is missing from your environment.")
		return
	}

	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session:", err)
		return
	}

	discord.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Printf("Logged in as %s (ID: %s)\n", s.State.User.Username, s.State.User.ID)

		command := &discordgo.ApplicationCommand{
			Name:        "factcheck",
			Description: "Fact check a statement",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "query",
					Description: "The statement or claim you want to fact check",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		}

		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", command)
		if err != nil {
			fmt.Printf("Error creating command: %v\n", err)
		}
	})

	discord.AddHandler(factCheckCommand)

	discord.Identify.Intents = discordgo.IntentsGuilds

	err = discord.Open()
	if err != nil {
		fmt.Println("Error opening Discord connection:", err)
		return
	}

	fmt.Println("Bot is running. Press Ctrl+C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc

	discord.Close()
}

func factCheckCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()
	if data.Name != "factcheck" {
		return
	}

	query := ""
	for _, option := range data.Options {
		if option.Name == "query" {
			query = option.StringValue()
			break
		}
	}

	if query == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please provide a query to fact-check.",
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	result := PerformFactCheck(query)

	sourcesJSON, err := json.Marshal(result.Sources)
	if err != nil {
		sourcesJSON = []byte("[]")
	}

	if _, err := AddFactCheck(query, result.Summary, string(sourcesJSON)); err != nil {
		fmt.Printf("Error saving to DB: %v\n", err)
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Fact Check: %s", query),
		Description: result.Summary,
		Color:       0x3498db,
	}

	if len(result.Sources) > 0 {
		var sourcesText string
		for idx, src := range result.Sources {
			if idx >= 5 {
				break
			}
			sourcesText += fmt.Sprintf("%d. [%s](%s)\n", idx+1, src.Title, src.URL)
		}
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Sources",
				Value:  sourcesText,
				Inline: false,
			},
		}
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		fmt.Printf("Error sending followup: %v\n", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Fact Check</title>
	<style>
		* { box-sizing: border-box; margin: 0; padding: 0; }
		body { background: #111; color: #eee; font-family: system-ui, sans-serif; min-height: 100vh; display: flex; justify-content: center; padding: 40px 20px; }
		.container { width: 100%; max-width: 700px; }
		h1 { font-size: 1.5rem; margin-bottom: 20px; color: #fff; }
		form { display: flex; gap: 10px; margin-bottom: 30px; }
		input { flex: 1; padding: 12px 16px; background: #222; border: 1px solid #333; color: #fff; font-size: 1rem; border-radius: 4px; }
		input:focus { outline: none; border-color: #555; }
		button { padding: 12px 24px; background: #333; color: #fff; border: 1px solid #444; cursor: pointer; font-size: 1rem; border-radius: 4px; }
		button:hover { background: #444; }
		button:disabled { opacity: 0.5; cursor: not-allowed; }
		.result { background: #1a1a1a; border: 1px solid #333; border-radius: 4px; padding: 20px; }
		.result.loading { color: #888; font-style: italic; }
		.result h2 { font-size: 1.1rem; margin-bottom: 12px; color: #fff; }
		.result p { line-height: 1.6; color: #ccc; white-space: pre-wrap; }
		.sources { margin-top: 20px; padding-top: 20px; border-top: 1px solid #333; }
		.sources h3 { font-size: 0.9rem; color: #888; margin-bottom: 10px; text-transform: uppercase; letter-spacing: 1px; }
		.sources a { color: #6ab0f3; text-decoration: none; display: block; margin-bottom: 8px; }
		.sources a:hover { text-decoration: underline; }
		.error { color: #e74c3c; }
	</style>
</head>
<body>
	<div class="container">
		<h1>Fact Check</h1>
		<form id="form">
			<input type="text" id="query" placeholder="Enter a claim to fact-check..." required autocomplete="off">
			<button type="submit" id="btn">Check</button>
		</form>
		<div id="result"></div>
	</div>
	<script>
		const form = document.getElementById('form');
		const query = document.getElementById('query');
		const btn = document.getElementById('btn');
		const result = document.getElementById('result');

		form.addEventListener('submit', async (e) => {
			e.preventDefault();
			const q = query.value.trim();
			if (!q) return;

			btn.disabled = true;
			result.innerHTML = '<div class="result loading">Checking...</div>';

			try {
				const res = await fetch('/api/factcheck', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ query: q })
				});
				const data = await res.json();

				if (data.error) {
					result.innerHTML = '<div class="result error">' + data.error + '</div>';
				} else {
					let html = '<div class="result"><h2>' + q + '</h2><p>' + data.summary + '</p>';
					if (data.sources && data.sources.length) {
						html += '<div class="sources"><h3>Sources</h3>';
						data.sources.forEach(s => {
							html += '<a href="' + s.url + '" target="_blank" rel="noopener">' + s.title + '</a>';
						});
						html += '</div>';
					}
					html += '</div>';
					result.innerHTML = html;
				}
			} catch (err) {
				result.innerHTML = '<div class="result error">Something went wrong</div>';
			}

			btn.disabled = false;
		});
	</script>
</body>
</html>`
	io.WriteString(w, html)
}

func handleFactCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	if req.Query == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "Query is required"})
		return
	}

	result := PerformFactCheck(req.Query)

	sourcesJSON, _ := json.Marshal(result.Sources)
	AddFactCheck(req.Query, result.Summary, string(sourcesJSON))

	json.NewEncoder(w).Encode(result)
}
