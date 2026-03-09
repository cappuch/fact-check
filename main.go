package main

import (
	"encoding/json"
	"fmt"
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
