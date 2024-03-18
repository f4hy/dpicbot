package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var apiKey string
var token string

var rollMap = map[string]int{
	":one: :red_circle:":    1,
	":two:":                 2,
	":three:":               3,
	":four: :green_circle:": 4,
}

// Define the structure for the request payload
type ImageGenerationRequest struct {
	Model   string `json:"model,omitempty"` // dall-e-2 or dall-e-3
	Prompt  string `json:"prompt"`
	N       int    `json:"n,omitempty"`       // number of images to generate
	Size    string `json:"size,omitempty"`    // 1024x1024, 1792x1024, 1024x1792 for dall-e-3
	Quality int    `json:"quality,omitempty"` // can specify hd for dall-e-3
	Style   int    `json:"style,omitempty"`   // must be vivd or natural
}

// Define the structure for the response
type ImageGenerationResponse struct {
	Created int `json:"created"`
	Data    []struct {
		URL           string `json:"url"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
}

type GPTPrompt struct {
	Model       string       `json:"model,omitempty"`       // gpt-4, gpt-3.5-turbo
	MaxTokens   int          `json:"max_tokens,omitempty"`  // 60
	N           int          `json:"n,omitempty"`           // 1
	Temperature float64      `json:"temperature,omitempty"` // 0-2
	Messages    []GPTMessage `json:"messages,omitempty"`
	Seed        int          `json:"seed,omitempty"` // 0-2147483647
}

type GPTResponse struct {
	Choices []struct {
		Index   int                    `json:"index,omitempty"` // 0, 1, 2, 3, 4
		Message map[string]interface{} `json:"message,omitempty"`
		// Message []GPTMessage `json:"message,omitempty"`
		// Message []map[string]interface{} `json:"message,omitempty"`
	} `json:"choices,omitempty"`
}

type GPTMessage struct {
	Role    string `json:"role,omitempty"`    // user, system
	Content string `json:"content,omitempty"` // previous message
}

func main() {
	// Set your Discord bot token from an env variable
	token = os.Getenv("DISCORD_BOT_TOKEN")
	// Set your OpenAI API key from an env variable
	apiKey = os.Getenv("OPENAI_API_KEY")
	if token == "" || apiKey == "" {
		fmt.Println("Please set the DISCORD_BOT_TOKEN and OPENAI_API_KEY environment variables.")
		return
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL+C to exit.")
	select {}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Message.Interaction == nil {
		return
	}
	fmt.Printf("%+v\n", m.Message)
	fmt.Printf("%+v\n", m.Message.Embeds[0])
	fmt.Printf("%+v\n", m.Message.Interaction.Name)         // roll
	fmt.Printf("%+v\n", m.Message.Embeds[0].Fields[0].Name) //one-four // count
	fmt.Printf("%+v\n", m.Message.Author.Username)          //one-four // count

	if m.Message.Author.Username == "Beyond 20" &&
		m.Message.Interaction.Name == "roll" &&
		len(m.Message.Embeds) > 0 &&
		len(m.Message.Embeds[0].Fields) > 0 &&
		m.Message.Embeds[0].Title == "Ds" {
		words := GeneratePrompt(m.Message.Embeds[0].Fields[0].Name)
		prompt := ""
		for i, line := range strings.Split(words, "\n") {
			prompt += line
			fmt.Printf("%d:%d\n", i, rollMap[m.Message.Embeds[0].Fields[0].Name])
			if i == rollMap[m.Message.Embeds[0].Fields[0].Name]-1 {
				break
			}
			prompt += " and "
		}

		imageURL := GenerateImage(prompt)
		response, err := http.Get(imageURL)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Failed to download the image.")
			return
		}
		defer response.Body.Close()

		// Create a temporary file to save the image
		file, err := os.CreateTemp("", "image.*.png")
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Failed to create a temporary file.")
			return
		}
		defer file.Close()

		// Copy the downloaded content into the temporary file
		_, err = io.Copy(file, response.Body)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Failed to save the image.")
			return
		}

		// Reset the file pointer to the beginning of the file
		file.Seek(0, 0)

		// Upload the image to the Discord channel
		_, err = s.ChannelFileSend(m.ChannelID, "image.png", file)
		s.ChannelMessageSend(m.ChannelID, prompt)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Failed to upload the image.")
			return
		}
		// if _, err := s.ChannelMessageSend(m.ChannelID, response); err != nil {
		// 	fmt.Println("Failed to send message,", err)
		// }
	}
}

func GeneratePrompt(dcount string) string {
	// Generate a random seed
	seed := rand.Intn(100) + 4
	payload := GPTPrompt{
		Model: "gpt-4", // Or any other suitable model
		Messages: []GPTMessage{
			{
				Role:    "user",
				Content: fmt.Sprintf("Think of %d Dungeons and dragons related words that start with the letter D and give me the last 4. The words can be about the game or stereotypical things that go on with people while playing it. Put each on a newline.", seed),
			},
		},
		MaxTokens:   60,  // Adjust as necessary
		Temperature: 0.7, // Adjust as necessary
		Seed:        seed,
	}

	// Marshal the payload into JSON
	requestBody, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("Error marshalling request:", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Fatal("Error creating request:", err)
	}

	// Set the required headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send the request using the default HTTP client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error sending request to OpenAI:", err)
	}
	defer resp.Body.Close()

	// Read and parse the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response body:", err)
	}

	fmt.Printf("%s\n", respBody)
	var openAIResp GPTResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		log.Fatal("Error unmarshalling response:", err)
	}

	// Output the result
	fmt.Printf("Response from OpenAI: %+v\n", openAIResp.Choices[0].Message["content"].(string))
	return openAIResp.Choices[0].Message["content"].(string)
	//return openAIResp.Choices[0].Message[0]["content"].(string)
}

func GenerateImage(prompt string) string {
	// Create a new instance of ImageGenerationRequest
	requestPayload := ImageGenerationRequest{
		Prompt: fmt.Sprintf("Make a single image that combines the following dungeons and dragons related things: %s", prompt),
		Model:  "dall-e-3",
		N:      1,
		Size:   "1024x1024",
	}

	// Marshal the request payload into JSON
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		panic(err)
	}

	// Create a new HTTP request to the OpenAI API
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/images/generations", bytes.NewBuffer(requestBody))
	if err != nil {
		panic(err)
	}

	// Set the required headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Make the request using the default HTTP client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Payload: %s\n", body)
	// Unmarshal the response body into the ImageGenerationResponse structure
	var responsePayload ImageGenerationResponse
	if err := json.Unmarshal(body, &responsePayload); err != nil {
		panic(err)
	}

	// Print the image URL
	fmt.Printf("Generated Image URL: %s\n", responsePayload.Data[0].URL)
	return responsePayload.Data[0].URL
}
