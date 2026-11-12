package youtrack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	testMailboxID       = "254-0"
	testMailRuleID      = "416-0"
	testMailboxPath     = "/" + mailboxesAPIPath + "/" + testMailboxID
	testMailRulePath    = "/" + mailRulesAPIPath + "/" + testMailRuleID
	testMailboxStatus   = "/" + mailboxIntegrationAPIPath + "/" + testMailboxID + "/" + mailboxStatusPathSegment
	fmtUnexpectedFields = "unexpected fields: %s"
)

// testMailbox is a mailbox as YouTrack 2026.2 answers it.
func testMailbox() Mailbox {
	return Mailbox{
		ID:                testMailboxID,
		Protocol:          &MailboxIDRef{ID: MailProtocolIMAPS, Type: "MailProtocol"},
		Host:              "imap.example.com",
		Port:              993,
		Login:             "support@example.com",
		SocketTimeOut:     60,
		ConnectionTimeOut: 60,
		Rules:             []MailboxIDRef{{ID: testMailRuleID, Type: mailRuleType}},
		Type:              mailboxType,
	}
}

// readJSONBody decodes a request body into a generic map, so a test can
// assert on which keys were sent and which were left out.
func readJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	body := map[string]any{}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("failed to decode body %s: %v", raw, err)
	}

	return body
}

func ptr[T any](v T) *T { return &v }

func TestListMailboxes(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != httpMethodGet {
			t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodGet)
		}
		if r.URL.Path != "/"+mailboxesAPIPath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("$top") != "10" || q.Get("$skip") != "20" {
			t.Errorf("unexpected pagination: %s", r.URL.RawQuery)
		}
		if q.Get("fields") != mailboxFields {
			t.Errorf(fmtUnexpectedFields, q.Get("fields"))
		}
		encodeJSON(t, w, []Mailbox{testMailbox()})
	})
	defer server.Close()

	got, err := client.ListMailboxes(context.Background(), 10, 20)
	if checkErr(t, err, false) {
		return
	}
	if len(got) != 1 || got[0].ID != testMailboxID || got[0].Protocol.ID != MailProtocolIMAPS || got[0].Rules[0].ID != testMailRuleID {
		t.Fatalf("unexpected mailboxes: %+v", got)
	}
}

func TestGetMailboxByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       int
		wantErr      bool
		wantNotFound bool
	}{
		{name: "found", status: http.StatusOK},
		{name: "missing", status: http.StatusNotFound, wantErr: true, wantNotFound: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != testMailboxPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				if tc.status != http.StatusOK {
					w.WriteHeader(tc.status)
					return
				}
				encodeJSON(t, w, testMailbox())
			})
			defer server.Close()

			got, err := client.GetMailboxByID(context.Background(), testMailboxID)
			if tc.wantNotFound && !IsNotFoundError(err) {
				t.Fatalf("expected a not-found error, got %v", err)
			}
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != testMailboxID {
				t.Fatalf(fmtUnexpectedID, got.ID, testMailboxID)
			}
		})
	}
}

func TestMailboxWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		wantPath string
		call     func(*Client) (*Mailbox, error)
		wantKeys map[string]any
		absent   []string
	}{
		{
			name:     "create IMAPS",
			wantPath: "/" + mailboxesAPIPath,
			call: func(c *Client) (*Mailbox, error) {
				return c.CreateMailbox(context.Background(), MailboxPayload{
					Protocol: &MailboxIDRef{ID: MailProtocolIMAPS},
					Host:     ptr("imap.example.com"),
					Port:     ptr(993),
					Login:    ptr("support@example.com"),
					Password: ptr("s3cret"),
				})
			},
			wantKeys: map[string]any{"$type": mailboxType, "host": "imap.example.com", "port": float64(993), "password": "s3cret"},
			absent:   []string{"microsoftClientSecret", "socketTimeOut", "sslKey"},
		},
		{
			name:     "partial update never sends a null password",
			wantPath: testMailboxPath,
			call: func(c *Client) (*Mailbox, error) {
				return c.UpdateMailbox(context.Background(), testMailboxID, MailboxPayload{
					SocketTimeOut:             ptr(30),
					ExchangeCompatibilityMode: ptr(false),
				})
			},
			wantKeys: map[string]any{"$type": mailboxType, "socketTimeOut": float64(30), "exchangeCompatibilityMode": false},
			absent:   []string{"password", "host", "port", "login", "microsoftClientSecret"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodPost {
					t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodPost)
				}
				if r.URL.Path != tc.wantPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				assertBodyKeys(t, readJSONBody(t, r), tc.wantKeys, tc.absent)
				encodeJSON(t, w, testMailbox())
			})
			defer server.Close()

			got, err := tc.call(client)
			if checkErr(t, err, false) {
				return
			}
			if got.ID != testMailboxID {
				t.Fatalf(fmtUnexpectedID, got.ID, testMailboxID)
			}
		})
	}
}

// assertBodyKeys checks that body carries want and none of absent.
func assertBodyKeys(t *testing.T, body, want map[string]any, absent []string) {
	t.Helper()

	for key, value := range want {
		got, ok := body[key]
		if !ok {
			t.Errorf("body is missing %q: %v", key, body)
			continue
		}
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(value)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("body[%q] = %s, want %s", key, gotJSON, wantJSON)
		}
	}
	for _, key := range absent {
		if _, ok := body[key]; ok {
			t.Errorf("body must not carry %q: %v", key, body)
		}
	}
}

func TestDeleteMailbox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		deleteStatus int
		// visibleReads is how many reads still see the mailbox after the delete.
		visibleReads int32
		// removingReads is how many reads after those answer that the removal
		// is in progress.
		removingReads int32
		wantErr       bool
	}{
		{name: "gone at once", deleteStatus: http.StatusOK},
		{name: "waits for an asynchronous delete", deleteStatus: http.StatusOK, visibleReads: 2},
		{name: "waits through the removal in progress", deleteStatus: http.StatusOK, visibleReads: 1, removingReads: 2},
		{name: "already gone", deleteStatus: http.StatusNotFound},
		{name: "server error", deleteStatus: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reads atomic.Int32
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != testMailboxPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				switch r.Method {
				case httpMethodDelete:
					w.WriteHeader(tc.deleteStatus)
				case httpMethodGet:
					read := reads.Add(1)
					switch {
					case read <= tc.visibleReads:
						encodeJSON(t, w, testMailbox())
					case read <= tc.visibleReads+tc.removingReads:
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = io.WriteString(w, "Mailbox["+testMailboxID+"] was removed.")
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				default:
					t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodDelete)
				}
			})
			defer server.Close()

			err := client.DeleteMailbox(context.Background(), testMailboxID)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if want := tc.visibleReads + tc.removingReads + 1; reads.Load() != want {
				t.Fatalf("expected %d reads, got %d", want, reads.Load())
			}
		})
	}
}

func TestMailboxFetchControl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		call        func(*Client) (*MailboxStatus, error)
		wantMethod  string
		wantRunning *bool
	}{
		{
			name: "status",
			call: func(c *Client) (*MailboxStatus, error) {
				return c.GetMailboxStatus(context.Background(), testMailboxID)
			},
			wantMethod: httpMethodGet,
		},
		{
			name: "start",
			call: func(c *Client) (*MailboxStatus, error) {
				return c.StartMailboxFetch(context.Background(), testMailboxID)
			},
			wantMethod:  httpMethodPost,
			wantRunning: ptr(true),
		},
		{
			name: "cancel",
			call: func(c *Client) (*MailboxStatus, error) {
				return c.CancelMailboxFetch(context.Background(), testMailboxID)
			},
			wantMethod:  httpMethodPost,
			wantRunning: ptr(false),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.wantMethod {
					t.Errorf(fmtUnexpectedMethod, r.Method, tc.wantMethod)
				}
				if r.URL.Path != testMailboxStatus {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				if tc.wantRunning != nil {
					assertBodyKeys(t, readJSONBody(t, r), map[string]any{"isRunning": *tc.wantRunning}, nil)
				}
				encodeJSON(t, w, MailboxStatus{ID: testMailboxID, Status: "Failed to retrieve messages", LastFetchStart: "moments ago"})
			})
			defer server.Close()

			got, err := tc.call(client)
			if checkErr(t, err, false) {
				return
			}
			if got.ID != testMailboxID || got.LastFetchStart != "moments ago" {
				t.Fatalf("unexpected status: %+v", got)
			}
		})
	}
}

func TestTestMailboxConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "passes", status: http.StatusOK},
		{name: "fails", status: http.StatusBadRequest, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/"+mailboxesAPIPath+"/"+mailboxTestConnectionSuffix {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				w.WriteHeader(tc.status)
				if tc.status != http.StatusOK {
					_, _ = io.WriteString(w, `{"error":"mailing_error","error_description":"Couldn't connect to host"}`)
				}
			})
			defer server.Close()

			err := client.TestMailboxConnection(context.Background(), MailboxPayload{Host: ptr("imap.example.com")})
			if checkErr(t, err, tc.wantErr) {
				return
			}
		})
	}
}

func TestMailboxCronSettings(t *testing.T) {
	t.Parallel()

	const expr = "0 0/5 * * * ?"

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+mailboxCronAPIPath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		if r.Method == httpMethodPost {
			assertBodyKeys(t, readJSONBody(t, r), map[string]any{"cronExpression": expr}, []string{"minCronIntervalForHosted"})
		}
		encodeJSON(t, w, MailboxCronSettings{CronExpression: expr, MinCronIntervalForHosted: 600})
	})
	defer server.Close()

	got, err := client.GetMailboxCronSettings(context.Background())
	if checkErr(t, err, false) {
		return
	}
	if got.CronExpression != expr || got.MinCronIntervalForHosted != 600 {
		t.Fatalf("unexpected cron settings: %+v", got)
	}

	got, err = client.UpdateMailboxCronSettings(context.Background(), expr)
	if checkErr(t, err, false) {
		return
	}
	if got.CronExpression != expr {
		t.Fatalf("unexpected cron settings: %+v", got)
	}
}

func TestGetMailboxLimitations(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+mailboxLimitationAPIPath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		encodeJSON(t, w, MailboxLimitations{MaxTimeOut: 300})
	})
	defer server.Close()

	got, err := client.GetMailboxLimitations(context.Background())
	if checkErr(t, err, false) {
		return
	}
	if got.MaxTimeOut != 300 {
		t.Fatalf("unexpected limitations: %+v", got)
	}
}

func TestListMailboxFolders(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testMailboxPath || r.URL.Query().Get("fields") != mailboxFoldersFields {
			t.Errorf("unexpected request: %s", r.URL)
		}
		encodeJSON(t, w, map[string]any{"id": testMailboxID, "folders": []string{"INBOX", "Support"}})
	})
	defer server.Close()

	got, err := client.ListMailboxFolders(context.Background(), testMailboxID)
	if checkErr(t, err, false) {
		return
	}
	if strings.Join(got, ",") != "INBOX,Support" {
		t.Fatalf("unexpected folders: %v", got)
	}
}

// testMailRule is a mail rule as YouTrack 2026.2 answers it.
func testMailRule() MailRule {
	return MailRule{
		ID:             testMailRuleID,
		Mailbox:        &MailboxIDRef{ID: testMailboxID, Type: mailboxType},
		Project:        &MailRuleProjectRef{ID: "0-0", ShortName: "DEMO", Type: "Project"},
		Maintainer:     &UserRef{ID: "2-1", Login: "admin", Type: "User"},
		Folder:         "INBOX",
		Policy:         &MailRulePolicy{Policy: MailRulePolicySenderOrNewUser, Type: "FromPolicy"},
		SpamFilterMode: &MailboxIDRef{ID: MailRuleSpamFilterMark, Type: "SpamFilterMode"},
		Filters: []MailRuleFilter{{
			Scope:         &MailboxIDRef{ID: MailRuleFilterScopeSubject, Type: "MailRuleFilterScope"},
			Patterns:      "urgent\nbug",
			AcceptMatched: true,
			Type:          "SimplePatternMailFilter",
		}},
		FilterAutoReplies:  true,
		OriginalTimestamps: true,
		Type:               mailRuleType,
	}
}

func TestListMailRules(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+mailRulesAPIPath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		if r.URL.Query().Get("fields") != mailRuleFields {
			t.Errorf(fmtUnexpectedFields, r.URL.Query().Get("fields"))
		}
		encodeJSON(t, w, []MailRule{testMailRule()})
	})
	defer server.Close()

	got, err := client.ListMailRules(context.Background(), 0, 0)
	if checkErr(t, err, false) {
		return
	}
	if len(got) != 1 || got[0].Project.ShortName != "DEMO" || got[0].Filters[0].Scope.ID != MailRuleFilterScopeSubject {
		t.Fatalf("unexpected rules: %+v", got)
	}
}

func TestMailRuleWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		wantPath string
		call     func(*Client) (*MailRule, error)
		wantKeys map[string]any
		absent   []string
	}{
		{
			name:     "create",
			wantPath: "/" + mailRulesAPIPath,
			call: func(c *Client) (*MailRule, error) {
				return c.CreateMailRule(context.Background(), MailRulePayload{
					Mailbox:    &MailboxIDRef{ID: testMailboxID},
					Project:    &MailboxIDRef{ID: "0-0"},
					Maintainer: &UserRef{ID: "2-1"},
					Folder:     ptr("INBOX"),
					Policy:     &MailRulePolicy{Policy: MailRulePolicySenderOrDefault, User: &UserRef{ID: "2-1"}},
				})
			},
			wantKeys: map[string]any{
				"$type":   mailRuleType,
				"mailbox": map[string]any{"id": testMailboxID},
				"project": map[string]any{"id": "0-0"},
				"policy":  map[string]any{"policy": MailRulePolicySenderOrDefault, "user": map[string]any{"id": "2-1"}},
			},
			absent: []string{"filters", "disabled", "replyTo"},
		},
		{
			name:     "update types the filters",
			wantPath: testMailRulePath,
			call: func(c *Client) (*MailRule, error) {
				return c.UpdateMailRule(context.Background(), testMailRuleID, MailRulePayload{
					Filters: &[]MailRuleFilter{{Scope: &MailboxIDRef{ID: MailRuleFilterScopeBody}, Patterns: "x"}},
				})
			},
			wantKeys: map[string]any{
				"filters": []any{map[string]any{
					"$type": simplePatternMailFilterType, "scope": map[string]any{"id": MailRuleFilterScopeBody},
					"patterns": "x", "acceptMatched": false,
				}},
			},
			absent: []string{"policy"},
		},
		{
			name:     "update clears filters, reply-to and the policy user",
			wantPath: testMailRulePath,
			call: func(c *Client) (*MailRule, error) {
				return c.UpdateMailRule(context.Background(), testMailRuleID, MailRulePayload{
					Disabled: ptr(true),
					ReplyTo:  ptr(""),
					Filters:  &[]MailRuleFilter{},
					Policy:   &MailRulePolicy{Policy: MailRulePolicySenderOrNewUser},
				})
			},
			wantKeys: map[string]any{
				"disabled": true,
				"replyTo":  "",
				"filters":  []any{},
				"policy":   map[string]any{"policy": MailRulePolicySenderOrNewUser, "user": nil},
			},
			absent: []string{"mailbox", "project", "maintainer", "folder"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodPost {
					t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodPost)
				}
				if r.URL.Path != tc.wantPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				assertBodyKeys(t, readJSONBody(t, r), tc.wantKeys, tc.absent)
				encodeJSON(t, w, testMailRule())
			})
			defer server.Close()

			got, err := tc.call(client)
			if checkErr(t, err, false) {
				return
			}
			if got.ID != testMailRuleID {
				t.Fatalf(fmtUnexpectedID, got.ID, testMailRuleID)
			}
		})
	}
}

func TestDeleteMailRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "deleted", status: http.StatusOK},
		{name: "already gone", status: http.StatusNotFound},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodDelete || r.URL.Path != testMailRulePath {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.status)
			})
			defer server.Close()

			checkErr(t, client.DeleteMailRule(context.Background(), testMailRuleID), tc.wantErr)
		})
	}
}

func TestMailboxRequestErrors(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, testInvalidJSON)
	})
	defer server.Close()

	if _, err := client.GetMailRuleByID(context.Background(), testMailRuleID); err == nil {
		t.Fatal(errExpectedError)
	}
	if _, err := client.ListMailboxes(context.Background(), 0, 0); err == nil {
		t.Fatal(errExpectedError)
	}
}
