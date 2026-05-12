package youtrack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	testWorkTypeID      = "65-5"
	testWorkTypeName    = "Investigation"
	errUnexpectedMethod = "unexpected method: %s"
)

func TestGetGlobalTimeTrackingSettings(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/admin/timeTrackingSettings") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		encodeJSON(t, w, GlobalTimeTrackingSettings{
			ID: "global",
			WorkTimeSettings: WorkTimeSettings{
				ID:             "64-0",
				MinutesADay:    480,
				WorkDays:       []int{1, 2, 3, 4, 5},
				FirstDayOfWeek: 1,
				DaysAWeek:      5,
			},
			WorkItemTypes: []WorkItemType{{ID: testWorkTypeID, Name: testWorkTypeName, AutoAttached: true}},
		})
	})
	defer server.Close()

	settings, err := client.GetGlobalTimeTrackingSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.WorkTimeSettings.MinutesADay != 480 {
		t.Fatalf("unexpected minutesADay: got %d want %d", settings.WorkTimeSettings.MinutesADay, 480)
	}
	if len(settings.WorkItemTypes) != 1 {
		t.Fatalf("unexpected work item types length: got %d want %d", len(settings.WorkItemTypes), 1)
	}
}

func handleUpdateWorkTimePost(t *testing.T, w http.ResponseWriter, r *http.Request, postCalled *bool) {
	t.Helper()
	*postCalled = true
	if !strings.HasPrefix(r.URL.Path, "/api/admin/timeTrackingSettings/workTimeSettings") {
		t.Fatalf("unexpected POST path: %s", r.URL.Path)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}
	if payload["minutesADay"] != float64(450) {
		t.Fatalf("unexpected minutesADay payload: %#v", payload["minutesADay"])
	}
	encodeJSON(t, w, WorkTimeSettings{})
}

func TestUpdateWorkTimeSettings(t *testing.T) {
	t.Parallel()

	postCalled := false

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleUpdateWorkTimePost(t, w, r, &postCalled)
		case http.MethodGet:
			encodeJSON(t, w, WorkTimeSettings{ID: "64-0", MinutesADay: 450, WorkDays: []int{1, 2, 3, 4, 5}, FirstDayOfWeek: 1, DaysAWeek: 5})
		default:
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
	})
	defer server.Close()

	updated, err := client.UpdateWorkTimeSettings(context.Background(), WorkTimeSettings{MinutesADay: 450, WorkDays: []int{1, 2, 3, 4, 5}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !postCalled {
		t.Fatal("expected POST request to be called")
	}

	if updated.MinutesADay != 450 {
		t.Fatalf("unexpected updated minutesADay: got %d want %d", updated.MinutesADay, 450)
	}
}

func TestWorkItemTypeCRUD(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			encodeJSON(t, w, []WorkItemType{{ID: testWorkTypeID, Name: testWorkTypeName, AutoAttached: true}})
		case http.MethodPost:
			if strings.HasSuffix(r.URL.Path, "/"+testWorkTypeID) {
				encodeJSON(t, w, WorkItemType{ID: testWorkTypeID, Name: "Implementation", AutoAttached: false})
				return
			}
			encodeJSON(t, w, WorkItemType{ID: testWorkTypeID, Name: testWorkTypeName, AutoAttached: true})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
	})
	defer server.Close()

	items, err := client.ListWorkItemTypes(context.Background())
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected item count: got %d want %d", len(items), 1)
	}

	created, err := client.CreateWorkItemType(context.Background(), WorkItemType{Name: testWorkTypeName, AutoAttached: true})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created item ID to be set")
	}

	updated, err := client.UpdateWorkItemType(context.Background(), WorkItemType{ID: testWorkTypeID, Name: "Implementation", AutoAttached: false})
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	if updated.Name != "Implementation" {
		t.Fatalf("unexpected updated name: got %q want %q", updated.Name, "Implementation")
	}

	if err := client.DeleteWorkItemType(context.Background(), testWorkTypeID); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
}
