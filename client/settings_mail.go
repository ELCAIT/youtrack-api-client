package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	mailPath = "%s/api/admin/globalSettings/notificationSettings?fields=emailSettings(isEnabled,mailProtocol,host,port,anonymous,login,from,replyTo)"
)

// GetMailServer - Returns list of mail servers.
func (c *Client) GetMailServer(ctx context.Context) (MailServer, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(mailPath, c.HostURL), nil)
	if err != nil {
		return MailServer{}, fmt.Errorf("failed to create get mail server request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return MailServer{}, fmt.Errorf("failed to get mail server: %w", err)
	}

	var response MailServerResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return MailServer{}, fmt.Errorf("failed to unmarshal mail server response: %w", err)
	}

	return response.MailServer, nil
}

// UpdateMailServer - Updates an existing mail server.
func (c *Client) UpdateMailServer(ctx context.Context, mailServer MailServer) (MailServer, error) {
	// Wrap the mail server in the expected request structure
	requestBody := map[string]interface{}{
		"emailSettings": mailServer,
	}

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return MailServer{}, fmt.Errorf("failed to marshal mail server request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(mailPath, c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return MailServer{}, fmt.Errorf("failed to create update mail server request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return MailServer{}, fmt.Errorf("failed to update mail server: %w", err)
	}

	var response MailServerResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return MailServer{}, fmt.Errorf("failed to unmarshal mail server response: %w", err)
	}

	return response.MailServer, nil
}
