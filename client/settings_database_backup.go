package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	backupPath             = "%s/api/admin/databaseBackup/settings?%s"
	backupFieldsQueryParam = "fields=id,location,filesToKeep,cronExpression,archiveFormat,isOn,availableDiskSpace,notifiedUsers(id,login,fullName,email),backupStatus(id,backupInProgress,backupCancelled,backupError)"
)

// GetBackupSettings returns backup settings.
func (c *Client) GetBackupSettings(ctx context.Context) (BackupSettings, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(backupPath, c.HostURL, backupFieldsQueryParam), nil)
	if err != nil {
		return BackupSettings{}, fmt.Errorf("failed to create get backup settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return BackupSettings{}, fmt.Errorf("failed to get backup settings: %w", err)
	}

	var response BackupSettings
	err = json.Unmarshal(body, &response)
	if err != nil {
		return BackupSettings{}, fmt.Errorf("failed to unmarshal backup settings response: %w", err)
	}

	return response, nil
}

// UpdateBackupSettings updates backup settings.
func (c *Client) UpdateBackupSettings(ctx context.Context, backupSettings BackupSettings) (BackupSettings, error) {
	rb, err := json.Marshal(backupSettings)
	if err != nil {
		return BackupSettings{}, fmt.Errorf("failed to marshal backup settings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(backupPath, c.HostURL, backupFieldsQueryParam), bytes.NewReader(rb))
	if err != nil {
		return BackupSettings{}, fmt.Errorf("failed to create update backup settings request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return BackupSettings{}, fmt.Errorf("failed to update backup settings: %w", err)
	}

	waitForAsyncProcessing()

	return c.GetBackupSettings(ctx)
}
