package mattermost

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yml openapi.json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// The basic information for a post to be created in Mattermost.
type Post struct {
	ChannelId string `json:"channel_id"`
	Message   string `json:"message"`
}

// Sends a message in Mattermost with the given markdown content.
//
// Requires that the MATTERMOST_URL, MATTERMOST_TOKEN, and MATTERMOST_CHANNEL
// environment variables are set.
func CreateMessage(message string) error {
	url := os.Getenv("MATTERMOST_URL") + "/api/v4/posts"
	token := os.Getenv("MATTERMOST_TOKEN")
	channelId := os.Getenv("MATTERMOST_CHANNEL")

	post := Post{
		ChannelId: channelId,
		Message:   message,
	}

	postBody, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("failed to marshal post: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create post, status code: %d", resp.StatusCode)
	}

	return nil
}
