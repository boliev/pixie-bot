package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Client struct {
	bot        *tgbotapi.BotAPI
	httpClient *http.Client
}

func NewClient(bot *tgbotapi.BotAPI) *Client {
	return &Client{
		bot: bot,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DownloadFile implements ports.FileFetcher.
func (c *Client) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	file, err := c.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, fmt.Errorf("get file info: %w", err)
	}

	url := file.Link(c.bot.Token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download file HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read file body: %w", err)
	}

	return data, nil
}

// SendPhoto sends raw image bytes as a photo message.
func (c *Client) SendPhoto(_ context.Context, chatID int64, data []byte) error {
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
		Name:  "result.png",
		Bytes: data,
	})

	_, err := c.bot.Send(photo)
	if err != nil {
		return fmt.Errorf("send photo: %w", err)
	}

	return nil
}
