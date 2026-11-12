package youtrack

// Mailbox protocols accepted by MailboxPayload.Protocol. POP and POPS are
// still accepted by YouTrack but deprecated in its UI.
const (
	MailProtocolIMAP      = "IMAP"
	MailProtocolIMAPS     = "IMAPS"
	MailProtocolOffice365 = "OFFICE365"
	MailProtocolPOP       = "POP"
	MailProtocolPOPS      = "POPS"
)

// Reporter policies of a mail rule (MailRulePolicy.Policy). SENDER_OR_DEFAULT
// and ALWAYS_PREDEFINED need MailRulePolicy.User.
const (
	MailRulePolicySenderOrNewUser  = "SENDER_OR_NEW_USER"
	MailRulePolicySenderOrDefault  = "SENDER_OR_DEFAULT"
	MailRulePolicyAlwaysPredefined = "ALWAYS_PREDEFINED"
)

// Spam filter modes of a mail rule (MailRule.SpamFilterMode).
const (
	MailRuleSpamFilterNone  = "NONE"
	MailRuleSpamFilterMark  = "MARK_TICKETS"
	MailRuleSpamFilterBlock = "BLOCK_TICKETS"
)

// Scopes a mail rule filter matches its patterns against (MailRuleFilter.Scope).
const (
	MailRuleFilterScopeSubject        = "SUBJECT"
	MailRuleFilterScopeBody           = "BODY"
	MailRuleFilterScopeSubjectAndBody = "SUBJECT_AND_BODY"
)

const (
	mailboxType                 = "Mailbox"
	mailRuleType                = "MailRule"
	simplePatternMailFilterType = "SimplePatternMailFilter"
)

// MailboxIDRef references a mailbox-integration entity, or one of its
// enumerated values (a protocol, a spam filter mode, a filter scope), by ID.
type MailboxIDRef struct {
	ID   string `json:"id"`
	Type string `json:"$type,omitempty"`
}

// MailboxSSLKeyRef references the SSL client key a mailbox authenticates with.
type MailboxSSLKeyRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Type string `json:"$type,omitempty"`
}

// Mailbox is a mail account YouTrack polls for emails, which its mail rules
// turn into issues and comments.
//
// The password and the Microsoft client secret are write-only: YouTrack never
// returns them, so they are not part of this read model (see MailboxPayload).
// For an OFFICE365 mailbox, Host and Login are empty and the account is
// MicrosoftMailboxAddress.
type Mailbox struct {
	ID                        string            `json:"id,omitempty"`
	Protocol                  *MailboxIDRef     `json:"protocol,omitempty"`
	Host                      string            `json:"host,omitempty"`
	Port                      int               `json:"port,omitempty"`
	Login                     string            `json:"login,omitempty"`
	SSLKey                    *MailboxSSLKeyRef `json:"sslKey,omitempty"`
	SocketTimeOut             int               `json:"socketTimeOut,omitempty"`
	ConnectionTimeOut         int               `json:"connectionTimeOut,omitempty"`
	ExchangeCompatibilityMode bool              `json:"exchangeCompatibilityMode,omitempty"`
	CloudDefault              bool              `json:"cloudDefault,omitempty"`
	MicrosoftMailboxAddress   string            `json:"microsoftMailboxAddress,omitempty"`
	MicrosoftClientID         string            `json:"microsoftClientId,omitempty"`
	MicrosoftTenantID         string            `json:"microsoftTenantId,omitempty"`
	Rules                     []MailboxIDRef    `json:"rules,omitempty"`
	Type                      string            `json:"$type,omitempty"`
}

// MailboxPayload is the request body for creating or updating a mailbox. A nil
// field is left out of the request, so an update changes only the fields that
// are set.
//
// YouTrack refuses an update that changes Host, Port or Login unless Password
// is sent in the same request ("Re-enter the credentials to change the
// integration URL").
//
// Password and MicrosoftClientSecret are never sent as null: YouTrack clears
// the stored secret on an explicit null, and the mailbox then fails to log in.
//
// SocketTimeOut and ConnectionTimeOut are in seconds, from 1 to the maximum
// GetMailboxLimitations reports (300 on a stock instance).
type MailboxPayload struct {
	Protocol                  *MailboxIDRef `json:"protocol,omitempty"`
	Host                      *string       `json:"host,omitempty"`
	Port                      *int          `json:"port,omitempty"`
	Login                     *string       `json:"login,omitempty"`
	Password                  *string       `json:"password,omitempty"` //nolint:gosec // G117: the mailbox password is intentionally sent to the API
	SSLKey                    *MailboxIDRef `json:"sslKey,omitempty"`
	SocketTimeOut             *int          `json:"socketTimeOut,omitempty"`
	ConnectionTimeOut         *int          `json:"connectionTimeOut,omitempty"`
	ExchangeCompatibilityMode *bool         `json:"exchangeCompatibilityMode,omitempty"`
	CloudDefault              *bool         `json:"cloudDefault,omitempty"`
	MicrosoftMailboxAddress   *string       `json:"microsoftMailboxAddress,omitempty"`
	MicrosoftClientID         *string       `json:"microsoftClientId,omitempty"`
	MicrosoftTenantID         *string       `json:"microsoftTenantId,omitempty"`
	MicrosoftClientSecret     *string       `json:"microsoftClientSecret,omitempty"` //nolint:gosec // G117: the Microsoft client secret is intentionally sent to the API
	Type                      string        `json:"$type,omitempty"`
}

// MailRulePolicy decides who becomes the reporter of an issue created from an
// email. User is the predefined account for SENDER_OR_DEFAULT and
// ALWAYS_PREDEFINED; it is sent as null when nil, which clears it.
type MailRulePolicy struct {
	Policy string   `json:"policy"`
	User   *UserRef `json:"user"`
	Type   string   `json:"$type,omitempty"`
}

// MailRuleFilter accepts or rejects emails whose Scope matches one of
// Patterns, one pattern per line.
type MailRuleFilter struct {
	Scope         *MailboxIDRef `json:"scope"`
	Patterns      string        `json:"patterns"`
	AcceptMatched bool          `json:"acceptMatched"`
	Type          string        `json:"$type,omitempty"`
}

// MailRuleProjectRef references the project a mail rule files emails into.
type MailRuleProjectRef struct {
	ID        string `json:"id"`
	ShortName string `json:"shortName,omitempty"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"$type,omitempty"`
}

// MailRule turns the emails a mailbox receives into issues and comments of
// one project.
//
// YouTrack's own UI also asks for filterByRegexes, applyFilterToBody,
// autoReplyRegexes and warning; YouTrack 2026.2 silently drops them on both
// read and write, so they are not modelled.
type MailRule struct {
	ID                           string              `json:"id,omitempty"`
	Mailbox                      *MailboxIDRef       `json:"mailbox,omitempty"`
	Project                      *MailRuleProjectRef `json:"project,omitempty"`
	Maintainer                   *UserRef            `json:"maintainer,omitempty"`
	Disabled                     bool                `json:"disabled,omitempty"`
	Folder                       string              `json:"folder,omitempty"`
	To                           string              `json:"to,omitempty"`
	ReplyTo                      string              `json:"replyTo,omitempty"`
	Policy                       *MailRulePolicy     `json:"policy,omitempty"`
	SpamFilterMode               *MailboxIDRef       `json:"spamFilterMode,omitempty"`
	Filters                      []MailRuleFilter    `json:"filters,omitempty"`
	FilterAutoReplies            bool                `json:"filterAutoReplies,omitempty"`
	NewIssueCommand              string              `json:"newIssueCommand,omitempty"`
	NewCommentCommand            string              `json:"newCommentCommand,omitempty"`
	DoNotDiscardBracketsContents bool                `json:"doNotDiscardBracketsContents,omitempty"`
	DeleteAfterFetch             bool                `json:"deleteAfterFetch,omitempty"`
	OriginalTimestamps           bool                `json:"originalTimestamps,omitempty"`
	PlainTextDescription         bool                `json:"plainTextDescription,omitempty"`
	FetchOldEmails               bool                `json:"fetchOldEmails,omitempty"`
	Type                         string              `json:"$type,omitempty"`
}

// MailRulePayload is the request body for creating or updating a mail rule. A
// nil field is left out of the request, so an update changes only the fields
// that are set. Creating a rule requires Mailbox, Project, Maintainer and
// Policy.
//
// A string field set to "" clears it. Filters set to a pointer to an empty
// slice removes every filter; a non-empty list replaces the existing ones.
// Setting Mailbox on an update moves the rule to another mailbox.
type MailRulePayload struct {
	Mailbox                      *MailboxIDRef     `json:"mailbox,omitempty"`
	Project                      *MailboxIDRef     `json:"project,omitempty"`
	Maintainer                   *UserRef          `json:"maintainer,omitempty"`
	Disabled                     *bool             `json:"disabled,omitempty"`
	Folder                       *string           `json:"folder,omitempty"`
	To                           *string           `json:"to,omitempty"`
	ReplyTo                      *string           `json:"replyTo,omitempty"`
	Policy                       *MailRulePolicy   `json:"policy,omitempty"`
	SpamFilterMode               *MailboxIDRef     `json:"spamFilterMode,omitempty"`
	Filters                      *[]MailRuleFilter `json:"filters,omitempty"`
	FilterAutoReplies            *bool             `json:"filterAutoReplies,omitempty"`
	NewIssueCommand              *string           `json:"newIssueCommand,omitempty"`
	NewCommentCommand            *string           `json:"newCommentCommand,omitempty"`
	DoNotDiscardBracketsContents *bool             `json:"doNotDiscardBracketsContents,omitempty"`
	DeleteAfterFetch             *bool             `json:"deleteAfterFetch,omitempty"`
	OriginalTimestamps           *bool             `json:"originalTimestamps,omitempty"`
	PlainTextDescription         *bool             `json:"plainTextDescription,omitempty"`
	FetchOldEmails               *bool             `json:"fetchOldEmails,omitempty"`
	Type                         string            `json:"$type,omitempty"`
}

// MailboxStatus reports the last and current fetch of a mailbox.
//
// LastFetchStart and LastFetchFinish are human-readable presentations such as
// "moments ago" or "—" (never fetched), not timestamps.
type MailboxStatus struct {
	ID              string `json:"id,omitempty"`
	IsRunning       bool   `json:"isRunning"`
	IsCanceling     bool   `json:"isCanceling"`
	Status          string `json:"status,omitempty"`
	StatusError     string `json:"statusError,omitempty"`
	LastFetchStart  string `json:"lastFetchStart,omitempty"`
	LastFetchFinish string `json:"lastFetchFinish,omitempty"`
	Type            string `json:"$type,omitempty"`
}

// MailboxCronSettings is the Quartz cron schedule on which YouTrack polls
// every mailbox. MinCronIntervalForHosted is the shortest interval, in
// seconds, YouTrack Cloud accepts; it is read-only.
type MailboxCronSettings struct {
	CronExpression           string `json:"cronExpression"`
	MinCronIntervalForHosted int    `json:"minCronIntervalForHosted,omitempty"`
	Type                     string `json:"$type,omitempty"`
}

// MailboxLimitations reports the limits YouTrack applies to mailboxes.
// MaxTimeOut is the largest socket and connection timeout, in seconds.
type MailboxLimitations struct {
	MaxTimeOut int    `json:"maxTimeOut"`
	Type       string `json:"$type,omitempty"`
}

// mailboxFolders is the folder list of a mailbox, as the folders projection of
// the mailbox entity returns it.
type mailboxFolders struct {
	Folders []string `json:"folders"`
}
