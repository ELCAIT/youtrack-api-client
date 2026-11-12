package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The mailbox integration endpoints are NOT part of the officially documented
// YouTrack REST API, and are absent from the instance's own /api/openapi.json.
// They are the endpoints YouTrack's administration UI (Administration >
// Integrations > Mailbox Integration) uses, so treat them like the apps
// endpoints in apps.go: best-effort, and verify them when upgrading YouTrack.
//
// Verified against YouTrack 2026.2 (build 19766). Behaviours this client
// depends on:
//   - Creating posts to the collection; updating posts to the entity, and is
//     partial: a field left out of the body keeps its value. Posting to the
//     collection with an ID creates a new entity rather than updating one.
//   - Passwords and the Microsoft client secret are write-only. An explicit
//     null clears them, so the payloads never send one.
//   - Deleting a mailbox deletes its rules too. When it has rules, the delete
//     is acknowledged before it is applied, so DeleteMailbox waits until the
//     mailbox is gone.
//     While that removal is in progress, reading the mailbox answers 500 with
//     the plain-text body "Mailbox[<id>] was removed." instead of a 404.
//   - A mail rule filter must carry $type SimplePatternMailFilter, or YouTrack
//     fails with 500 InstantiationException; the client sets it.
//   - Unknown names in `fields` are silently dropped, like the rest of the API.

const (
	mailboxIntegrationAPIPath   = "api/admin/integrations/mailbox"
	mailboxesAPIPath            = mailboxIntegrationAPIPath + "/mailboxes"
	mailRulesAPIPath            = mailboxIntegrationAPIPath + "/rules"
	mailboxCronAPIPath          = mailboxIntegrationAPIPath + "/cron"
	mailboxLimitationAPIPath    = mailboxIntegrationAPIPath + "/limitation"
	mailboxStatusPathSegment    = "status"
	mailboxTestConnectionSuffix = "testConnection"

	mailboxFields = "id,protocol(id,$type),host,port,login,sslKey(id,name,$type),socketTimeOut,connectionTimeOut," +
		"exchangeCompatibilityMode,cloudDefault,microsoftMailboxAddress,microsoftClientId,microsoftTenantId," +
		"rules(id,$type),$type"
	mailRuleFields = "id,mailbox(id,$type),project(id,shortName,name,$type),maintainer(id,login,name,$type),disabled," +
		"folder,to,replyTo,policy(policy,user(id,login,name,$type),$type),spamFilterMode(id,$type)," +
		"filters(scope(id,$type),patterns,acceptMatched,$type),filterAutoReplies,newIssueCommand,newCommentCommand," +
		"doNotDiscardBracketsContents,deleteAfterFetch,originalTimestamps,plainTextDescription,fetchOldEmails,$type"
	mailboxStatusFields      = "id,isRunning,isCanceling,status,statusError,lastFetchStart,lastFetchFinish,$type"
	mailboxCronFields        = "cronExpression,minCronIntervalForHosted,$type"
	mailboxLimitationFields  = "maxTimeOut,$type"
	mailboxFoldersFields     = "id,folders"
	errMailboxRequestFailure = "failed to %s: %w"

	// mailboxRemovalInProgress is the body YouTrack answers a read of a mailbox
	// with while its delete is being applied.
	mailboxRemovalInProgress = "was removed"
)

// mailboxStatusUpdate starts or cancels a fetch.
type mailboxStatusUpdate struct {
	IsRunning bool `json:"isRunning"`
}

// sendMailboxRequest sends one request to a mailbox integration endpoint and
// decodes the response into T. payload is marshalled as the body when it is not
// nil. action names the operation in error messages ("create mailbox").
func sendMailboxRequest[T any](ctx context.Context, c *Client, method, endpoint string, payload any, action string) (*T, error) {
	var body io.Reader
	if payload != nil {
		rb, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal %s payload: %w", action, err)
		}
		body = bytes.NewReader(rb)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", action, err)
	}

	respBody, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf(errMailboxRequestFailure, action, err)
	}

	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s response: %w", action, err)
	}

	return &result, nil
}

// ListMailboxes returns the mailboxes YouTrack polls, with the IDs of their
// rules. Pass 0 for top and skip to use the server's default pagination.
func (c *Client) ListMailboxes(ctx context.Context, top, skip int) ([]Mailbox, error) {
	endpoint := c.buildURL(mailboxesAPIPath, nil, paginatedQuery(mailboxFields, top, skip))
	mailboxes, err := sendMailboxRequest[[]Mailbox](ctx, c, httpMethodGet, endpoint, nil, "list mailboxes")
	if err != nil {
		return nil, err
	}

	return *mailboxes, nil
}

// GetMailboxByID reads one mailbox. A missing mailbox is reported as an error
// IsNotFoundError recognises.
func (c *Client) GetMailboxByID(ctx context.Context, mailboxID string) (*Mailbox, error) {
	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxID}, fieldsQuery(mailboxFields))
	return sendMailboxRequest[Mailbox](ctx, c, httpMethodGet, endpoint, nil, "get mailbox")
}

// CreateMailbox adds a mailbox. An IMAP(S) mailbox needs Protocol, Host, Port,
// Login and Password; an OFFICE365 one needs Protocol,
// MicrosoftMailboxAddress, MicrosoftClientID, MicrosoftTenantID and
// MicrosoftClientSecret. YouTrack does not connect to the server on create;
// use TestMailboxConnection for that.
func (c *Client) CreateMailbox(ctx context.Context, payload MailboxPayload) (*Mailbox, error) {
	payload.Type = mailboxType
	endpoint := c.buildURL(mailboxesAPIPath, nil, fieldsQuery(mailboxFields))
	return sendMailboxRequest[Mailbox](ctx, c, httpMethodPost, endpoint, payload, "create mailbox")
}

// UpdateMailbox changes the fields of a mailbox that payload sets. Changing
// Host, Port or Login requires Password in the same payload.
func (c *Client) UpdateMailbox(ctx context.Context, mailboxID string, payload MailboxPayload) (*Mailbox, error) {
	payload.Type = mailboxType
	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxID}, fieldsQuery(mailboxFields))
	return sendMailboxRequest[Mailbox](ctx, c, httpMethodPost, endpoint, payload, "update mailbox")
}

// DeleteMailbox deletes a mailbox and every mail rule attached to it. Deleting
// a mailbox that is already gone succeeds. It returns once the mailbox is no
// longer readable, because YouTrack applies the delete of a mailbox with rules
// after acknowledging it.
func (c *Client) DeleteMailbox(ctx context.Context, mailboxID string) error {
	err := deleteByID(ctx, c, mailboxID, deleteConfig{
		APIPath:   mailboxesAPIPath,
		ErrCreate: "failed to create delete mailbox request: %w",
		ErrFetch:  "failed to delete mailbox: %w",
	})
	if err != nil {
		return err
	}

	stillExists := func(ctx context.Context) (bool, error) {
		_, err := c.GetMailboxByID(ctx, mailboxID)
		switch {
		case IsNotFoundError(err):
			return false, nil
		case isMailboxRemovalInProgress(err):
			return true, nil
		default:
			return err == nil, err
		}
	}

	if _, err := readBackAfterWrite(ctx, stillExists, func(exists bool) bool { return !exists }); err != nil {
		return fmt.Errorf("failed to confirm mailbox deletion: %w", err)
	}

	return nil
}

// isMailboxRemovalInProgress reports whether err is YouTrack's answer to a
// read of a mailbox whose delete it has acknowledged but not yet applied.
func isMailboxRemovalInProgress(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) &&
		httpErr.StatusCode == http.StatusInternalServerError &&
		strings.Contains(string(httpErr.Body), mailboxRemovalInProgress)
}

// TestMailboxConnection asks YouTrack to log in to the mail server with the
// given settings, without saving them. It returns nil when the login
// succeeded; otherwise the error carries YouTrack's reason, such as
// "Couldn't connect to host".
func (c *Client) TestMailboxConnection(ctx context.Context, payload MailboxPayload) error {
	payload.Type = mailboxType
	rb, err := json.Marshal(payload) //nolint:gosec // G117: the password is intentionally sent to test the login
	if err != nil {
		return fmt.Errorf("failed to marshal test mailbox connection payload: %w", err)
	}

	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxTestConnectionSuffix}, nil)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return fmt.Errorf("failed to create test mailbox connection request: %w", err)
	}

	if _, err := c.doRequest(req); err != nil {
		return fmt.Errorf("mailbox connection test failed: %w", err)
	}

	return nil
}

// ListMailboxFolders returns the folders of a mailbox, as YouTrack reads them
// from the mail server. It is empty when YouTrack cannot connect.
func (c *Client) ListMailboxFolders(ctx context.Context, mailboxID string) ([]string, error) {
	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxID}, fieldsQuery(mailboxFoldersFields))
	folders, err := sendMailboxRequest[mailboxFolders](ctx, c, httpMethodGet, endpoint, nil, "list mailbox folders")
	if err != nil {
		return nil, err
	}

	return folders.Folders, nil
}

// GetMailboxStatus reports whether a mailbox is being fetched and how its last
// fetch went.
func (c *Client) GetMailboxStatus(ctx context.Context, mailboxID string) (*MailboxStatus, error) {
	endpoint := c.buildURL(mailboxIntegrationAPIPath, []string{mailboxID, mailboxStatusPathSegment}, fieldsQuery(mailboxStatusFields))
	return sendMailboxRequest[MailboxStatus](ctx, c, httpMethodGet, endpoint, nil, "get mailbox status")
}

// StartMailboxFetch fetches a mailbox now instead of waiting for the next
// scheduled poll. The fetch runs in the background; follow it with
// GetMailboxStatus.
func (c *Client) StartMailboxFetch(ctx context.Context, mailboxID string) (*MailboxStatus, error) {
	return c.setMailboxFetchRunning(ctx, mailboxID, true, "start mailbox fetch")
}

// CancelMailboxFetch cancels a running fetch of a mailbox.
func (c *Client) CancelMailboxFetch(ctx context.Context, mailboxID string) (*MailboxStatus, error) {
	return c.setMailboxFetchRunning(ctx, mailboxID, false, "cancel mailbox fetch")
}

func (c *Client) setMailboxFetchRunning(ctx context.Context, mailboxID string, running bool, action string) (*MailboxStatus, error) {
	endpoint := c.buildURL(mailboxIntegrationAPIPath, []string{mailboxID, mailboxStatusPathSegment}, fieldsQuery(mailboxStatusFields))
	return sendMailboxRequest[MailboxStatus](ctx, c, httpMethodPost, endpoint, mailboxStatusUpdate{IsRunning: running}, action)
}

// GetMailboxCronSettings reads the schedule on which YouTrack polls every
// mailbox.
func (c *Client) GetMailboxCronSettings(ctx context.Context) (*MailboxCronSettings, error) {
	endpoint := c.buildURL(mailboxCronAPIPath, nil, fieldsQuery(mailboxCronFields))
	return sendMailboxRequest[MailboxCronSettings](ctx, c, httpMethodGet, endpoint, nil, "get mailbox cron settings")
}

// UpdateMailboxCronSettings sets the Quartz cron expression on which YouTrack
// polls every mailbox, such as "0 0/2 * * * ?" for every two minutes.
// YouTrack rejects an invalid expression.
func (c *Client) UpdateMailboxCronSettings(ctx context.Context, cronExpression string) (*MailboxCronSettings, error) {
	endpoint := c.buildURL(mailboxCronAPIPath, nil, fieldsQuery(mailboxCronFields))
	payload := MailboxCronSettings{CronExpression: cronExpression}
	return sendMailboxRequest[MailboxCronSettings](ctx, c, httpMethodPost, endpoint, payload, "update mailbox cron settings")
}

// GetMailboxLimitations reads the limits YouTrack applies to mailbox settings.
func (c *Client) GetMailboxLimitations(ctx context.Context) (*MailboxLimitations, error) {
	endpoint := c.buildURL(mailboxLimitationAPIPath, nil, fieldsQuery(mailboxLimitationFields))
	return sendMailboxRequest[MailboxLimitations](ctx, c, httpMethodGet, endpoint, nil, "get mailbox limitations")
}

// ListMailRules returns the mail rules of every mailbox. Pass 0 for top and
// skip to use the server's default pagination.
func (c *Client) ListMailRules(ctx context.Context, top, skip int) ([]MailRule, error) {
	endpoint := c.buildURL(mailRulesAPIPath, nil, paginatedQuery(mailRuleFields, top, skip))
	rules, err := sendMailboxRequest[[]MailRule](ctx, c, httpMethodGet, endpoint, nil, "list mail rules")
	if err != nil {
		return nil, err
	}

	return *rules, nil
}

// GetMailRuleByID reads one mail rule. A missing rule is reported as an error
// IsNotFoundError recognises.
func (c *Client) GetMailRuleByID(ctx context.Context, ruleID string) (*MailRule, error) {
	endpoint := c.buildURL(mailRulesAPIPath, []string{ruleID}, fieldsQuery(mailRuleFields))
	return sendMailboxRequest[MailRule](ctx, c, httpMethodGet, endpoint, nil, "get mail rule")
}

// withMailRuleTypes sets the $type YouTrack needs on a rule and its filters.
// The filter list is copied, so the caller's payload is left untouched.
func withMailRuleTypes(payload MailRulePayload) MailRulePayload {
	payload.Type = mailRuleType
	if payload.Filters == nil {
		return payload
	}

	filters := make([]MailRuleFilter, len(*payload.Filters))
	for i, filter := range *payload.Filters {
		if filter.Type == "" {
			filter.Type = simplePatternMailFilterType
		}
		filters[i] = filter
	}
	payload.Filters = &filters

	return payload
}

// CreateMailRule adds a mail rule to a mailbox. Mailbox, Project, Maintainer
// and Policy are required.
func (c *Client) CreateMailRule(ctx context.Context, payload MailRulePayload) (*MailRule, error) {
	payload = withMailRuleTypes(payload)
	endpoint := c.buildURL(mailRulesAPIPath, nil, fieldsQuery(mailRuleFields))
	return sendMailboxRequest[MailRule](ctx, c, httpMethodPost, endpoint, payload, "create mail rule")
}

// UpdateMailRule changes the fields of a mail rule that payload sets.
func (c *Client) UpdateMailRule(ctx context.Context, ruleID string, payload MailRulePayload) (*MailRule, error) {
	payload = withMailRuleTypes(payload)
	endpoint := c.buildURL(mailRulesAPIPath, []string{ruleID}, fieldsQuery(mailRuleFields))
	return sendMailboxRequest[MailRule](ctx, c, httpMethodPost, endpoint, payload, "update mail rule")
}

// DeleteMailRule deletes a mail rule. Deleting a rule that is already gone
// succeeds.
func (c *Client) DeleteMailRule(ctx context.Context, ruleID string) error {
	return deleteByID(ctx, c, ruleID, deleteConfig{
		APIPath:   mailRulesAPIPath,
		ErrCreate: "failed to create delete mail rule request: %w",
		ErrFetch:  "failed to delete mail rule: %w",
	})
}
