package youtrack

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	appsIntegrationRunEnv     = "YOUTRACK_RUN_APPS_INTEGRATION_TESTS"
	appsIntegrationAllProjEnv = "YOUTRACK_RUN_APPS_ALL_PROJECTS_TEST"
	appsIntegrationAppNameEnv = "YOUTRACK_TEST_APP_NAME"
	appsIntegrationLeaderEnv  = "YOUTRACK_TEST_PROJECT_LEADER"

	defaultIntegrationLeader = "admin"
)

// requireAppsIntegrationConfig gates the apps suite behind its own environment
// variable on top of the usual integration switch. The apps endpoints are not
// officially documented (see apps.go), so a run against an instance whose
// YouTrack version dropped or reshaped them should be an explicit opt-in rather
// than something the standard integration run trips over.
func requireAppsIntegrationConfig(t *testing.T) (*Client, context.Context) {
	t.Helper()

	if os.Getenv(appsIntegrationRunEnv) != "1" {
		t.Skipf("skipping apps integration tests: set %s=1 to enable", appsIntegrationRunEnv)
	}

	return requireIntegrationConfig(t)
}

// resolveIntegrationApp picks the app to exercise: the one named by
// YOUTRACK_TEST_APP_NAME when set, otherwise the first app the instance
// reports. Apps cannot be installed over the REST API, so the suite works with
// whatever is already present and skips when there is nothing to work with.
func resolveIntegrationApp(t *testing.T, ctx context.Context, client *Client) *App {
	t.Helper()

	if name := strings.TrimSpace(os.Getenv(appsIntegrationAppNameEnv)); name != "" {
		app, err := client.GetAppByName(ctx, name)
		if err != nil {
			t.Fatalf("failed to resolve app %q from %s: %v", name, appsIntegrationAppNameEnv, err)
		}

		return app
	}

	apps, err := client.ListApps(ctx, 1, 0)
	if err != nil {
		t.Fatalf("failed to list apps: %v", err)
	}
	if len(apps) == 0 {
		t.Skipf("skipping apps integration tests: no apps installed on this instance (set %s to pick one)", appsIntegrationAppNameEnv)
	}

	return &apps[0]
}

// createIntegrationProject creates a throwaway project and registers its
// deletion, so activation is exercised against a project that starts with no
// app attached and leaves no trace behind.
func createIntegrationProject(t *testing.T, ctx context.Context, client *Client, stamp string) *Project {
	t.Helper()

	leaderLogin := strings.TrimSpace(os.Getenv(appsIntegrationLeaderEnv))
	if leaderLogin == "" {
		leaderLogin = defaultIntegrationLeader
	}

	leader, err := client.GetUserByLogin(ctx, leaderLogin)
	if err != nil {
		t.Skipf("skipping apps integration tests: cannot resolve project leader %q (set %s): %v",
			leaderLogin, appsIntegrationLeaderEnv, err)
	}

	project, err := client.CreateProject(ctx, ProjectCreatePayload{
		Name:        "IT Apps " + stamp,
		ShortName:   "ITAPP" + stamp,
		Description: "throwaway project for the apps activation integration test",
		Leader:      &UserRef{ID: leader.Id},
	})
	if err != nil {
		t.Fatalf("failed to create integration project: %v", err)
	}

	t.Cleanup(func() {
		if err := client.DeleteProject(context.Background(), project.ID); err != nil {
			t.Errorf("failed to delete integration project %s: %v", project.ID, err)
		}
	})

	return project
}

func integrationStamp() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
}

// TestIntegrationYouTrackAppProjectActivation walks the full per-project
// activation lifecycle against a live instance: attach, enable, disable,
// re-enable, and detach, checking idempotency at every step.
//
// The subtests share the same app and project and must run in order, so they
// are deliberately not parallel.
func TestIntegrationYouTrackAppProjectActivation(t *testing.T) {
	client, ctx := requireAppsIntegrationConfig(t)

	app := resolveIntegrationApp(t, ctx, client)
	project := createIntegrationProject(t, ctx, client, integrationStamp())
	t.Logf("exercising app %s (%s) against project %s (%s)", app.Name, app.ID, project.Name, project.ID)

	t.Cleanup(func() {
		// Guard against a mid-test failure leaving the app attached.
		if err := client.DetachAppFromProject(context.Background(), app.ID, project.ID); err != nil {
			t.Errorf("failed to detach app during cleanup: %v", err)
		}
	})

	t.Run("app is readable by id and by name", func(t *testing.T) {
		byID, err := client.GetAppByID(ctx, app.ID)
		if err != nil {
			t.Fatalf("failed to get app by id: %v", err)
		}
		if byID.ID != app.ID || byID.Name != app.Name {
			t.Fatalf("unexpected app by id: got %+v, want id %s name %s", byID, app.ID, app.Name)
		}

		byName, err := client.GetAppByName(ctx, app.Name)
		if err != nil {
			t.Fatalf("failed to get app by name: %v", err)
		}
		if byName.ID != app.ID {
			t.Fatalf(fmtUnexpectedID, byName.ID, app.ID)
		}
	})

	t.Run("fresh project has no usage", func(t *testing.T) {
		usage, err := client.GetAppUsageForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to read app usage: %v", err)
		}
		if usage != nil {
			t.Fatalf("expected no usage on a fresh project, got %+v", usage)
		}
	})

	var usageID string

	t.Run("enable attaches and enables the app", func(t *testing.T) {
		usage, err := client.EnableAppForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to enable app for project: %v", err)
		}
		assertUsageEnabled(t, usage, project.ID, true)
		usageID = usage.ID

		reread, err := client.GetAppUsageForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to re-read app usage: %v", err)
		}
		assertUsageEnabled(t, reread, project.ID, true)
	})

	t.Run("enable is idempotent", func(t *testing.T) {
		usage, err := client.EnableAppForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to re-enable app for project: %v", err)
		}
		assertUsageEnabled(t, usage, project.ID, true)
		if usage.ID != usageID {
			t.Fatalf("enable created a second usage: got %s, want %s", usage.ID, usageID)
		}
	})

	t.Run("usage shows up in the app usage list", func(t *testing.T) {
		usages, err := client.ListAppUsages(ctx, app.ID)
		if err != nil {
			t.Fatalf("failed to list app usages: %v", err)
		}
		if findAppUsageForProject(usages, project.ID) == nil {
			t.Fatalf("project %s missing from the app usage list (%d entries)", project.ID, len(usages))
		}
	})

	t.Run("disable turns the app off but keeps it attached", func(t *testing.T) {
		usage, err := client.DisableAppForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to disable app for project: %v", err)
		}
		assertUsageEnabled(t, usage, project.ID, false)

		reread, err := client.GetAppUsageForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to re-read app usage: %v", err)
		}
		if reread == nil {
			t.Fatal("disable detached the app instead of only disabling it")
		}
		assertUsageEnabled(t, reread, project.ID, false)
	})

	t.Run("disable is idempotent", func(t *testing.T) {
		usage, err := client.DisableAppForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to re-disable app for project: %v", err)
		}
		assertUsageEnabled(t, usage, project.ID, false)
		if usage.ID != usageID {
			t.Fatalf("disable replaced the usage: got %s, want %s", usage.ID, usageID)
		}
	})

	t.Run("re-enable turns a disabled usage back on", func(t *testing.T) {
		usage, err := client.EnableAppForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to re-enable a disabled app: %v", err)
		}
		assertUsageEnabled(t, usage, project.ID, true)
		if usage.ID != usageID {
			t.Fatalf("re-enable replaced the usage: got %s, want %s", usage.ID, usageID)
		}
	})

	t.Run("detach removes the app from the project", func(t *testing.T) {
		if err := client.DetachAppFromProject(ctx, app.ID, project.ID); err != nil {
			t.Fatalf("failed to detach app from project: %v", err)
		}

		usage, err := client.GetAppUsageForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("failed to read app usage after detach: %v", err)
		}
		if usage != nil {
			t.Fatalf("expected no usage after detach, got %+v", usage)
		}
	})

	t.Run("detach is idempotent", func(t *testing.T) {
		if err := client.DetachAppFromProject(ctx, app.ID, project.ID); err != nil {
			t.Fatalf("detaching an already-detached app returned an error: %v", err)
		}
	})

	t.Run("disabling a detached app is a no-op", func(t *testing.T) {
		usage, err := client.DisableAppForProject(ctx, app.ID, project.ID)
		if err != nil {
			t.Fatalf("disabling a detached app returned an error: %v", err)
		}
		if usage != nil {
			t.Fatalf("expected nil usage for a detached app, got %+v", usage)
		}
	})
}

func assertUsageEnabled(t *testing.T, usage *AppUsage, projectID string, wantEnabled bool) {
	t.Helper()

	if usage == nil {
		t.Fatal("expected an app usage, got nil")
	}
	if usage.ID == "" {
		t.Fatal("app usage has an empty id")
	}
	if usage.Project == nil || usage.Project.ID != projectID {
		t.Fatalf("app usage points at the wrong project: %+v", usage.Project)
	}
	if usage.Enabled != wantEnabled {
		t.Fatalf("unexpected enabled state: got %t, want %t", usage.Enabled, wantEnabled)
	}
}

// TestIntegrationYouTrackAppAllProjectsActivation exercises
// EnableAppForAllProjects. It touches every project on the instance, so it sits
// behind its own switch on top of the apps switch. The pre-existing usages are
// captured first and restored afterwards, but run it against a throwaway
// instance rather than production all the same.
func TestIntegrationYouTrackAppAllProjectsActivation(t *testing.T) {
	client, ctx := requireAppsIntegrationConfig(t)

	if os.Getenv(appsIntegrationAllProjEnv) != "1" {
		t.Skipf("skipping all-projects apps test: set %s=1 to enable (it touches every project)", appsIntegrationAllProjEnv)
	}

	app := resolveIntegrationApp(t, ctx, client)

	projects, err := client.ListProjects(ctx, 0, 0)
	if err != nil {
		t.Fatalf("failed to list projects: %v", err)
	}
	if len(projects) == 0 {
		t.Skip("skipping all-projects apps test: instance has no projects")
	}

	before, err := client.ListAppUsages(ctx, app.ID)
	if err != nil {
		t.Fatalf("failed to capture the app usages baseline: %v", err)
	}
	t.Cleanup(func() { restoreAppUsages(t, client, app.ID, before) })

	usages, err := client.EnableAppForAllProjects(ctx, app.ID)
	if err != nil {
		t.Fatalf("failed to enable app for all projects: %v", err)
	}
	if len(usages) != len(projects) {
		t.Fatalf("unexpected usages count: got %d, want %d", len(usages), len(projects))
	}

	after, err := client.ListAppUsages(ctx, app.ID)
	if err != nil {
		t.Fatalf("failed to list app usages after enabling: %v", err)
	}
	for i := range projects {
		usage := findAppUsageForProject(after, projects[i].ID)
		if usage == nil {
			t.Fatalf("project %s (%s) has no usage after EnableAppForAllProjects", projects[i].ID, projects[i].Name)
		}
		if !usage.Enabled {
			t.Fatalf("project %s (%s) is attached but not enabled", projects[i].ID, projects[i].Name)
		}
	}
}

// restoreAppUsages puts the app back into the usage state captured in before:
// usages that did not exist are detached, and those that did get their original
// enabled flag back.
func restoreAppUsages(t *testing.T, client *Client, appID string, before []AppUsage) {
	t.Helper()

	ctx := context.Background()

	current, err := client.ListAppUsages(ctx, appID)
	if err != nil {
		t.Errorf("failed to list app usages during restore: %v", err)
		return
	}

	for i := range current {
		if current[i].Project == nil {
			continue
		}

		projectID := current[i].Project.ID
		original := findAppUsageForProject(before, projectID)

		if original == nil {
			if err := client.DeleteAppUsage(ctx, appID, current[i].ID); err != nil {
				t.Errorf("failed to detach app from project %s during restore: %v", projectID, err)
			}
			continue
		}

		if original.Enabled != current[i].Enabled {
			if _, err := client.SetAppUsageEnabled(ctx, appID, current[i].ID, original.Enabled); err != nil {
				t.Errorf("failed to restore enabled=%t for project %s: %v", original.Enabled, projectID, err)
			}
		}
	}
}
