package youtrack

import (
	"context"
	"os"
	"slices"
	"testing"
)

const (
	mailboxIntegrationRunEnv = "YOUTRACK_RUN_MAILBOX_INTEGRATION_TESTS"

	// integrationMailHost never resolves (RFC 2606), so the suite never reaches
	// a real mail server and the connection test fails fast.
	integrationMailHost     = "imap.youtrack-client.invalid"
	integrationMailPassword = "integration-secret"
)

// requireMailboxIntegrationConfig gates the mailbox suite behind its own
// switch, because the endpoints are undocumented (see mailbox.go) and the
// suite briefly changes the instance-wide mailbox poll schedule.
func requireMailboxIntegrationConfig(t *testing.T) (*Client, context.Context) {
	t.Helper()

	if os.Getenv(mailboxIntegrationRunEnv) != "1" {
		t.Skipf("skipping mailbox integration tests: set %s=1 to enable", mailboxIntegrationRunEnv)
	}

	return requireIntegrationConfig(t)
}

// createIntegrationMailbox creates an IMAPS mailbox pointing at a host that
// does not exist, and registers its deletion.
func createIntegrationMailbox(t *testing.T, ctx context.Context, client *Client, login string) *Mailbox {
	t.Helper()

	mailbox, err := client.CreateMailbox(ctx, MailboxPayload{
		Protocol:          &MailboxIDRef{ID: MailProtocolIMAPS},
		Host:              ptr(integrationMailHost),
		Port:              ptr(993),
		Login:             ptr(login),
		Password:          ptr(integrationMailPassword),
		SocketTimeOut:     ptr(5),
		ConnectionTimeOut: ptr(5),
	})
	if err != nil {
		t.Fatalf("failed to create mailbox: %v", err)
	}

	t.Cleanup(func() {
		if err := client.DeleteMailbox(context.Background(), mailbox.ID); err != nil {
			t.Errorf("failed to delete mailbox %s: %v", mailbox.ID, err)
		}
	})

	return mailbox
}

func TestIntegrationYouTrackMailboxLifecycle(t *testing.T) {
	client, ctx := requireMailboxIntegrationConfig(t)
	stamp := integrationStamp()

	limits, err := client.GetMailboxLimitations(ctx)
	if err != nil || limits.MaxTimeOut <= 0 {
		t.Fatalf("unexpected limitations %+v: %v", limits, err)
	}

	mailbox := createIntegrationMailbox(t, ctx, client, "it-"+stamp+"@example.com")
	if mailbox.Protocol == nil || mailbox.Protocol.ID != MailProtocolIMAPS || mailbox.Host != integrationMailHost || mailbox.Port != 993 {
		t.Fatalf("unexpected created mailbox: %+v", mailbox)
	}

	got, err := client.GetMailboxByID(ctx, mailbox.ID)
	if err != nil || got.Login != mailbox.Login {
		t.Fatalf("unexpected read of mailbox %s: %+v, %v", mailbox.ID, got, err)
	}

	all, err := client.ListAllMailboxes(ctx)
	if err != nil {
		t.Fatalf("failed to list mailboxes: %v", err)
	}
	if !slices.ContainsFunc(all, func(m Mailbox) bool { return m.ID == mailbox.ID }) {
		t.Fatalf("mailbox %s missing from %+v", mailbox.ID, all)
	}

	// A partial update keeps every field it does not set.
	updated, err := client.UpdateMailbox(ctx, mailbox.ID, MailboxPayload{SocketTimeOut: ptr(7), ExchangeCompatibilityMode: ptr(true)})
	if err != nil {
		t.Fatalf("failed to update mailbox: %v", err)
	}
	if updated.SocketTimeOut != 7 || !updated.ExchangeCompatibilityMode || updated.Host != integrationMailHost || updated.Port != 993 {
		t.Fatalf("unexpected partially updated mailbox: %+v", updated)
	}

	// Changing the server needs the password again.
	if _, err := client.UpdateMailbox(ctx, mailbox.ID, MailboxPayload{Port: ptr(994)}); err == nil {
		t.Fatal("expected changing the port without the password to fail")
	}
	updated, err = client.UpdateMailbox(ctx, mailbox.ID, MailboxPayload{Port: ptr(994), Password: ptr(integrationMailPassword)})
	if err != nil || updated.Port != 994 {
		t.Fatalf("unexpected update with password: %+v, %v", updated, err)
	}

	if _, err := client.UpdateMailbox(ctx, mailbox.ID, MailboxPayload{SocketTimeOut: ptr(limits.MaxTimeOut + 1)}); err == nil {
		t.Fatal("expected a timeout above the limit to be rejected")
	}

	status, err := client.GetMailboxStatus(ctx, mailbox.ID)
	if err != nil || status.ID != mailbox.ID || status.IsRunning {
		t.Fatalf("unexpected status: %+v, %v", status, err)
	}

	err = client.TestMailboxConnection(ctx, MailboxPayload{
		Protocol:          &MailboxIDRef{ID: MailProtocolIMAPS},
		Host:              ptr(integrationMailHost),
		Port:              ptr(993),
		Login:             ptr(mailbox.Login),
		Password:          ptr(integrationMailPassword),
		SocketTimeOut:     ptr(5),
		ConnectionTimeOut: ptr(5),
	})
	if err == nil {
		t.Fatal("expected the connection test against a non-existent host to fail")
	}
}

func TestIntegrationYouTrackMailRuleLifecycle(t *testing.T) {
	client, ctx := requireMailboxIntegrationConfig(t)
	stamp := integrationStamp()

	project := createIntegrationProject(t, ctx, client, stamp)
	mailbox := createIntegrationMailbox(t, ctx, client, "it-rule-"+stamp+"@example.com")
	maintainer := &UserRef{ID: project.Leader.ID}

	rule, err := client.CreateMailRule(ctx, MailRulePayload{
		Mailbox:           &MailboxIDRef{ID: mailbox.ID},
		Project:           &MailboxIDRef{ID: project.ID},
		Maintainer:        maintainer,
		Folder:            ptr("INBOX"),
		Policy:            &MailRulePolicy{Policy: MailRulePolicySenderOrNewUser},
		SpamFilterMode:    &MailboxIDRef{ID: MailRuleSpamFilterMark},
		FilterAutoReplies: ptr(true),
	})
	if err != nil {
		t.Fatalf("failed to create mail rule: %v", err)
	}
	t.Cleanup(func() {
		if err := client.DeleteMailRule(context.Background(), rule.ID); err != nil {
			t.Errorf("failed to delete mail rule %s: %v", rule.ID, err)
		}
	})

	if rule.Mailbox.ID != mailbox.ID || rule.Project.ID != project.ID || rule.Folder != "INBOX" || !rule.FilterAutoReplies ||
		rule.SpamFilterMode.ID != MailRuleSpamFilterMark || rule.Policy.Policy != MailRulePolicySenderOrNewUser {
		t.Fatalf("unexpected created rule: %+v", rule)
	}

	withRule, err := client.GetMailboxByID(ctx, mailbox.ID)
	if err != nil || !slices.ContainsFunc(withRule.Rules, func(r MailboxIDRef) bool { return r.ID == rule.ID }) {
		t.Fatalf("mailbox %s does not list rule %s: %+v, %v", mailbox.ID, rule.ID, withRule, err)
	}

	updated, err := client.UpdateMailRule(ctx, rule.ID, MailRulePayload{
		Disabled:        ptr(true),
		ReplyTo:         ptr("reply-" + stamp + "@example.com"),
		NewIssueCommand: ptr("tag mail"),
		Policy:          &MailRulePolicy{Policy: MailRulePolicySenderOrDefault, User: maintainer},
		SpamFilterMode:  &MailboxIDRef{ID: MailRuleSpamFilterBlock},
		Filters: &[]MailRuleFilter{{
			Scope: &MailboxIDRef{ID: MailRuleFilterScopeSubject}, Patterns: "urgent\nbug", AcceptMatched: true,
		}},
	})
	if err != nil {
		t.Fatalf("failed to update mail rule: %v", err)
	}
	if !updated.Disabled || updated.ReplyTo == "" || updated.NewIssueCommand != "tag mail" || updated.Folder != "INBOX" ||
		updated.Policy.User == nil || updated.Policy.User.ID != maintainer.ID || updated.SpamFilterMode.ID != MailRuleSpamFilterBlock ||
		len(updated.Filters) != 1 || updated.Filters[0].Patterns != "urgent\nbug" {
		t.Fatalf("unexpected updated rule: %+v", updated)
	}

	cleared, err := client.UpdateMailRule(ctx, rule.ID, MailRulePayload{
		ReplyTo:         ptr(""),
		NewIssueCommand: ptr(""),
		Filters:         &[]MailRuleFilter{},
		Policy:          &MailRulePolicy{Policy: MailRulePolicySenderOrNewUser},
	})
	if err != nil {
		t.Fatalf("failed to clear mail rule fields: %v", err)
	}
	if cleared.ReplyTo != "" || cleared.NewIssueCommand != "" || len(cleared.Filters) != 0 || cleared.Policy.User != nil || !cleared.Disabled {
		t.Fatalf("unexpected cleared rule: %+v", cleared)
	}

	rules, err := client.ListAllMailRules(ctx)
	if err != nil || !slices.ContainsFunc(rules, func(r MailRule) bool { return r.ID == rule.ID }) {
		t.Fatalf("rule %s missing from the rule list: %v", rule.ID, err)
	}

	if err := client.DeleteMailRule(ctx, rule.ID); err != nil {
		t.Fatalf("failed to delete mail rule: %v", err)
	}
	if err := client.DeleteMailRule(ctx, rule.ID); err != nil {
		t.Fatalf("deleting a deleted mail rule must succeed: %v", err)
	}
	if _, err := client.GetMailRuleByID(ctx, rule.ID); !IsNotFoundError(err) {
		t.Fatalf("expected the deleted rule to be gone, got %v", err)
	}
}

func TestIntegrationYouTrackMailboxDeleteCascadesRules(t *testing.T) {
	client, ctx := requireMailboxIntegrationConfig(t)
	stamp := integrationStamp()

	project := createIntegrationProject(t, ctx, client, stamp)
	mailbox := createIntegrationMailbox(t, ctx, client, "it-cascade-"+stamp+"@example.com")

	rule, err := client.CreateMailRule(ctx, MailRulePayload{
		Mailbox:    &MailboxIDRef{ID: mailbox.ID},
		Project:    &MailboxIDRef{ID: project.ID},
		Maintainer: &UserRef{ID: project.Leader.ID},
		Policy:     &MailRulePolicy{Policy: MailRulePolicySenderOrNewUser},
	})
	if err != nil {
		t.Fatalf("failed to create mail rule: %v", err)
	}

	if err := client.DeleteMailbox(ctx, mailbox.ID); err != nil {
		t.Fatalf("failed to delete mailbox: %v", err)
	}
	if _, err := client.GetMailboxByID(ctx, mailbox.ID); !IsNotFoundError(err) {
		t.Fatalf("DeleteMailbox returned before the mailbox was gone: %v", err)
	}
	if _, err := client.GetMailRuleByID(ctx, rule.ID); !IsNotFoundError(err) {
		t.Fatalf("expected the rule to be deleted with its mailbox, got %v", err)
	}
}

func TestIntegrationYouTrackMailboxCronSettings(t *testing.T) {
	client, ctx := requireMailboxIntegrationConfig(t)

	original, err := client.GetMailboxCronSettings(ctx)
	if err != nil || original.CronExpression == "" {
		t.Fatalf("unexpected cron settings %+v: %v", original, err)
	}
	t.Cleanup(func() {
		if _, err := client.UpdateMailboxCronSettings(context.Background(), original.CronExpression); err != nil {
			t.Errorf("failed to restore mailbox cron expression %q: %v", original.CronExpression, err)
		}
	})

	const expr = "0 0/7 * * * ?"
	updated, err := client.UpdateMailboxCronSettings(ctx, expr)
	if err != nil || updated.CronExpression != expr {
		t.Fatalf("unexpected cron update %+v: %v", updated, err)
	}

	if _, err := client.UpdateMailboxCronSettings(ctx, "not a cron expression"); err == nil {
		t.Fatal("expected an invalid cron expression to be rejected")
	}
}
