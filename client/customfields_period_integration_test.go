package youtrack

import (
	"context"
	"testing"
)

const (
	periodIntegrationEmptyText = "No estimate"
	customFieldDefaultsType    = "CustomFieldDefaults"
)

// createIntegrationPeriodField creates a throwaway period custom field and
// registers its deletion. It is registered before the project that uses it, so
// the cleanup, which runs in reverse, deletes the project first.
func createIntegrationPeriodField(t *testing.T, ctx context.Context, client *Client, stamp string) *CustomField {
	t.Helper()

	canBeEmpty := false
	emptyFieldText := periodIntegrationEmptyText
	field, err := client.CreateCustomField(ctx, CustomFieldUpsertRequest{
		Name:      "IT Period " + stamp,
		FieldType: &FieldType{ID: PeriodFieldTypeID},
		FieldDefaults: &CustomFieldDefaultsUpsertModel{
			CanBeEmpty:     &canBeEmpty,
			EmptyFieldText: &emptyFieldText,
		},
	})
	if err != nil {
		t.Fatalf("failed to create period custom field: %v", err)
	}

	t.Cleanup(func() {
		if err := client.DeleteCustomField(context.Background(), field.ID); err != nil {
			t.Errorf("failed to delete period custom field %s: %v", field.ID, err)
		}
	})

	return field
}

func assertPeriodFieldDefaults(t *testing.T, field *CustomField) {
	t.Helper()

	if field.FieldType.ID != PeriodFieldTypeID {
		t.Fatalf("unexpected field type: got %q, want %q", field.FieldType.ID, PeriodFieldTypeID)
	}

	defaults := field.FieldDefaults
	if defaults == nil {
		t.Fatal("expected period field defaults, got none")
	}
	if defaults.Type != customFieldDefaultsType {
		t.Fatalf("unexpected field defaults type: got %q, want %q", defaults.Type, customFieldDefaultsType)
	}
	if defaults.CanBeEmpty || defaults.EmptyFieldText != periodIntegrationEmptyText {
		t.Fatalf("defaults not applied: canBeEmpty=%t emptyFieldText=%q", defaults.CanBeEmpty, defaults.EmptyFieldText)
	}
	if defaults.Bundle != nil {
		t.Fatalf("period field defaults unexpectedly carry a bundle: %+v", defaults.Bundle)
	}
}

// TestIntegrationYouTrackPeriodCustomFieldLifecycle creates a period field,
// attaches it to a project and uses it as the project's time tracking
// estimate, which is what period fields are for.
func TestIntegrationYouTrackPeriodCustomFieldLifecycle(t *testing.T) {
	client, ctx := requireIntegrationConfig(t)
	stamp := integrationStamp()

	field := createIntegrationPeriodField(t, ctx, client, stamp)
	assertPeriodFieldDefaults(t, field)

	project := createIntegrationProject(t, ctx, client, stamp)

	attached, err := client.AddProjectCustomField(ctx, project.ID, ProjectCustomFieldUpsertPayload{
		Field: &CustomFieldIDRef{ID: field.ID},
		Type:  PeriodProjectCustomFieldType,
	})
	if err != nil {
		t.Fatalf("failed to attach period field to project: %v", err)
	}
	if attached.Type != PeriodProjectCustomFieldType {
		t.Fatalf("unexpected project field type: got %q, want %q", attached.Type, PeriodProjectCustomFieldType)
	}
	if attached.Bundle != nil {
		t.Fatalf("period project field unexpectedly carries a bundle: %+v", attached.Bundle)
	}

	settings, err := client.UpdateProjectTimeTrackingSettings(ctx, project.ID, ProjectTimeTrackingUpdatePayload{
		Enabled:  true,
		Estimate: &ProjectCustomFieldTimeRef{ID: attached.ID},
	})
	if err != nil {
		t.Fatalf("failed to set the period field as the estimate: %v", err)
	}
	if settings.Estimate == nil || settings.Estimate.ID != attached.ID {
		t.Fatalf("estimate not set to the period field: %+v", settings.Estimate)
	}
}
