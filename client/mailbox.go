package youtrack

import (
	"context"
	"errors"
	"fmt"
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
//     the plain-text body "Mailbox[<id>] was removed." instead of a 404;
//     GetMailboxByID reports it as errMailboxRemovalInProgress.
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
	mailboxStatusFields     = "id,isRunning,isCanceling,status,statusError,lastFetchStart,lastFetchFinish,$type"
	mailboxCronFields       = "cronExpression,minCronIntervalForHosted,$type"
	mailboxLimitationFields = "maxTimeOut,$type"
	mailboxFoldersFields    = "id,folders"

	// mailboxRemovedFormat is the body YouTrack answers a request for a
	// mailbox with while its delete is being applied.
	mailboxRemovedFormat = "Mailbox[%s] was removed"

	mailboxIDKind  = "mailbox"
	mailRuleIDKind = "mail rule"
)

// errMailboxRemovalInProgress is returned by GetMailboxByID for a mailbox
// whose delete YouTrack has acknowledged but not yet applied. It wraps
// ErrNotFound, so IsNotFound treats the mailbox as gone.
var errMailboxRemovalInProgress = fmt.Errorf("mailbox removal in progress: %w", ErrNotFound)

// mailboxStatusUpdate starts or cancels a fetch.
type mailboxStatusUpdate struct {
	IsRunning bool `json:"isRunning"`
}

// requireID rejects an empty ID. buildURL turns it into the collection URL,
// where a POST creates a new entity instead of updating one.
func requireID(kind, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s id must not be empty", kind)
	}

	return nil
}

// ListMailboxes returns the mailboxes YouTrack polls, with the IDs of their
// rules. Pass 0 for top and skip to use the server's default pagination.
func (c *Client) ListMailboxes(ctx context.Context, top, skip int) ([]Mailbox, error) {
	endpoint := c.buildURL(mailboxesAPIPath, nil, paginatedQuery(mailboxFields, top, skip))
	mailboxes, err := sendAndDecode[[]Mailbox](ctx, c, httpMethodGet, endpoint, nil, "list mailboxes")
	if err != nil {
		return nil, err
	}

	return *mailboxes, nil
}

// GetMailboxByID reads one mailbox. A missing mailbox, or one whose delete
// YouTrack is still applying, is reported as an error IsNotFound recognises.
func (c *Client) GetMailboxByID(ctx context.Context, mailboxID string) (*Mailbox, error) {
	if err := requireID(mailboxIDKind, mailboxID); err != nil {
		return nil, err
	}

	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxID}, fieldsQuery(mailboxFields))
	mailbox, err := sendAndDecode[Mailbox](ctx, c, httpMethodGet, endpoint, nil, "get mailbox")
	if isMailboxRemovalInProgress(err, mailboxID) {
		return nil, fmt.Errorf("mailbox %q: %w: %w", mailboxID, errMailboxRemovalInProgress, err)
	}

	return mailbox, err
}

// CreateMailbox adds a mailbox. An IMAP(S) mailbox needs Protocol, Host, Port,
// Login and Password; an OFFICE365 one needs Protocol,
// MicrosoftMailboxAddress, MicrosoftClientID, MicrosoftTenantID and
// MicrosoftClientSecret. YouTrack does not connect to the server on create;
// use TestMailboxConnection for that.
func (c *Client) CreateMailbox(ctx context.Context, payload MailboxPayload) (*Mailbox, error) {
	payload.Type = mailboxType
	endpoint := c.buildURL(mailboxesAPIPath, nil, fieldsQuery(mailboxFields))
	return sendAndDecode[Mailbox](ctx, c, httpMethodPost, endpoint, payload, "create mailbox")
}

// UpdateMailbox changes the fields of a mailbox that payload sets. Changing
// Host, Port or Login requires Password in the same payload.
func (c *Client) UpdateMailbox(ctx context.Context, mailboxID string, payload MailboxPayload) (*Mailbox, error) {
	if err := requireID(mailboxIDKind, mailboxID); err != nil {
		return nil, err
	}

	payload.Type = mailboxType
	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxID}, fieldsQuery(mailboxFields))
	return sendAndDecode[Mailbox](ctx, c, httpMethodPost, endpoint, payload, "update mailbox")
}

// DeleteMailbox deletes a mailbox and every mail rule attached to it. Deleting
// a mailbox that is already gone succeeds. It returns once the mailbox is no
// longer readable, because YouTrack applies the delete of a mailbox with rules
// after acknowledging it, and fails if the mailbox is still there after
// asyncPollTimeout.
func (c *Client) DeleteMailbox(ctx context.Context, mailboxID string) error {
	if err := requireID(mailboxIDKind, mailboxID); err != nil {
		return err
	}

	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxID}, nil)
	_, err := sendJSON(ctx, c, httpMethodDelete, endpoint, nil, "delete mailbox")
	switch {
	case IsNotFoundError(err):
		// Already gone: there is no removal to wait for.
		return nil
	case err != nil && !isMailboxRemovalInProgress(err, mailboxID):
		return err
	}

	exists, err := readBackAfterWrite(ctx, c.mailboxStillExists(mailboxID), func(exists bool) bool { return !exists })
	if err != nil {
		return fmt.Errorf("failed to confirm mailbox deletion: %w", err)
	}
	if exists {
		return fmt.Errorf("mailbox %q still exists %s after it was deleted", mailboxID, asyncPollTimeout)
	}

	return nil
}

// mailboxStillExists reads a mailbox and reports whether it is still there,
// counting a removal in progress as still there.
func (c *Client) mailboxStillExists(mailboxID string) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		_, err := c.GetMailboxByID(ctx, mailboxID)
		switch {
		case errors.Is(err, errMailboxRemovalInProgress):
			return true, nil
		case IsNotFound(err):
			return false, nil
		case err != nil:
			return false, err
		default:
			return true, nil
		}
	}
}

// isMailboxRemovalInProgress reports whether err is YouTrack's answer to a
// request for mailboxID while its delete is acknowledged but not yet applied.
func isMailboxRemovalInProgress(err error, mailboxID string) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) &&
		httpErr.StatusCode == http.StatusInternalServerError &&
		strings.Contains(string(httpErr.Body), fmt.Sprintf(mailboxRemovedFormat, mailboxID))
}

// TestMailboxConnection asks YouTrack to log in to the mail server with the
// given settings, without saving them. It returns nil when the login
// succeeded; otherwise the error carries YouTrack's reason, such as
// "Couldn't connect to host".
func (c *Client) TestMailboxConnection(ctx context.Context, payload MailboxPayload) error {
	payload.Type = mailboxType
	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxTestConnectionSuffix}, nil)
	_, err := sendJSON(ctx, c, httpMethodPost, endpoint, payload, "test mailbox connection")

	return err
}

// ListMailboxFolders returns the folders of a mailbox, as YouTrack reads them
// from the mail server. It is empty when YouTrack cannot connect.
func (c *Client) ListMailboxFolders(ctx context.Context, mailboxID string) ([]string, error) {
	if err := requireID(mailboxIDKind, mailboxID); err != nil {
		return nil, err
	}

	endpoint := c.buildURL(mailboxesAPIPath, []string{mailboxID}, fieldsQuery(mailboxFoldersFields))
	folders, err := sendAndDecode[mailboxFolders](ctx, c, httpMethodGet, endpoint, nil, "list mailbox folders")
	if err != nil {
		return nil, err
	}

	return folders.Folders, nil
}

// GetMailboxStatus reports whether a mailbox is being fetched and how its last
// fetch went.
func (c *Client) GetMailboxStatus(ctx context.Context, mailboxID string) (*MailboxStatus, error) {
	if err := requireID(mailboxIDKind, mailboxID); err != nil {
		return nil, err
	}

	endpoint := c.buildURL(mailboxIntegrationAPIPath, []string{mailboxID, mailboxStatusPathSegment}, fieldsQuery(mailboxStatusFields))
	return sendAndDecode[MailboxStatus](ctx, c, httpMethodGet, endpoint, nil, "get mailbox status")
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
	if err := requireID(mailboxIDKind, mailboxID); err != nil {
		return nil, err
	}

	endpoint := c.buildURL(mailboxIntegrationAPIPath, []string{mailboxID, mailboxStatusPathSegment}, fieldsQuery(mailboxStatusFields))
	return sendAndDecode[MailboxStatus](ctx, c, httpMethodPost, endpoint, mailboxStatusUpdate{IsRunning: running}, action)
}

// GetMailboxCronSettings reads the schedule on which YouTrack polls every
// mailbox.
func (c *Client) GetMailboxCronSettings(ctx context.Context) (*MailboxCronSettings, error) {
	endpoint := c.buildURL(mailboxCronAPIPath, nil, fieldsQuery(mailboxCronFields))
	return sendAndDecode[MailboxCronSettings](ctx, c, httpMethodGet, endpoint, nil, "get mailbox cron settings")
}

// UpdateMailboxCronSettings sets the Quartz cron expression on which YouTrack
// polls every mailbox, such as "0 0/2 * * * ?" for every two minutes.
// YouTrack rejects an invalid expression.
func (c *Client) UpdateMailboxCronSettings(ctx context.Context, cronExpression string) (*MailboxCronSettings, error) {
	endpoint := c.buildURL(mailboxCronAPIPath, nil, fieldsQuery(mailboxCronFields))
	payload := MailboxCronSettings{CronExpression: cronExpression}
	return sendAndDecode[MailboxCronSettings](ctx, c, httpMethodPost, endpoint, payload, "update mailbox cron settings")
}

// GetMailboxLimitations reads the limits YouTrack applies to mailbox settings.
func (c *Client) GetMailboxLimitations(ctx context.Context) (*MailboxLimitations, error) {
	endpoint := c.buildURL(mailboxLimitationAPIPath, nil, fieldsQuery(mailboxLimitationFields))
	return sendAndDecode[MailboxLimitations](ctx, c, httpMethodGet, endpoint, nil, "get mailbox limitations")
}

// ListMailRules returns the mail rules of every mailbox. Pass 0 for top and
// skip to use the server's default pagination.
func (c *Client) ListMailRules(ctx context.Context, top, skip int) ([]MailRule, error) {
	endpoint := c.buildURL(mailRulesAPIPath, nil, paginatedQuery(mailRuleFields, top, skip))
	rules, err := sendAndDecode[[]MailRule](ctx, c, httpMethodGet, endpoint, nil, "list mail rules")
	if err != nil {
		return nil, err
	}

	return *rules, nil
}

// GetMailRuleByID reads one mail rule. A missing rule is reported as an error
// IsNotFoundError recognises.
func (c *Client) GetMailRuleByID(ctx context.Context, ruleID string) (*MailRule, error) {
	if err := requireID(mailRuleIDKind, ruleID); err != nil {
		return nil, err
	}

	endpoint := c.buildURL(mailRulesAPIPath, []string{ruleID}, fieldsQuery(mailRuleFields))
	return sendAndDecode[MailRule](ctx, c, httpMethodGet, endpoint, nil, "get mail rule")
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
	return sendAndDecode[MailRule](ctx, c, httpMethodPost, endpoint, payload, "create mail rule")
}

// UpdateMailRule changes the fields of a mail rule that payload sets.
func (c *Client) UpdateMailRule(ctx context.Context, ruleID string, payload MailRulePayload) (*MailRule, error) {
	if err := requireID(mailRuleIDKind, ruleID); err != nil {
		return nil, err
	}

	payload = withMailRuleTypes(payload)
	endpoint := c.buildURL(mailRulesAPIPath, []string{ruleID}, fieldsQuery(mailRuleFields))
	return sendAndDecode[MailRule](ctx, c, httpMethodPost, endpoint, payload, "update mail rule")
}

// DeleteMailRule deletes a mail rule. Deleting a rule that is already gone
// succeeds.
func (c *Client) DeleteMailRule(ctx context.Context, ruleID string) error {
	if err := requireID(mailRuleIDKind, ruleID); err != nil {
		return err
	}

	return deleteByID(ctx, c, ruleID, deleteConfig{
		APIPath:   mailRulesAPIPath,
		ErrCreate: "failed to create delete mail rule request: %w",
		ErrFetch:  "failed to delete mail rule: %w",
	})
}
