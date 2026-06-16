package youtrack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	integrationHostEnv   = "YOUTRACK_BASE_URL"
	integrationTokenEnv  = "YOUTRACK_TOKEN"
	integrationRunEnv    = "YOUTRACK_RUN_INTEGRATION_TESTS"
	hubIntegrationRunEnv = "YOUTRACK_RUN_HUB_INTEGRATION_TESTS"
	integrationUserPass  = "YOUTRACK_TEST_USER_PASSWORD"
)

func requireIntegrationConfig(t *testing.T) (*Client, context.Context) {
	t.Helper()

	if os.Getenv(integrationRunEnv) != "1" {
		t.Skipf("skipping integration tests: set %s=1 to enable", integrationRunEnv)
	}

	host := strings.TrimSpace(os.Getenv(integrationHostEnv))
	token := strings.TrimSpace(os.Getenv(integrationTokenEnv))
	if host == "" || token == "" {
		t.Skipf("skipping integration tests: set %s and %s", integrationHostEnv, integrationTokenEnv)
	}

	client, err := NewClient(host, token)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	return client, ctx
}

func requireHubIntegrationConfig(t *testing.T) (*Client, context.Context) {
	t.Helper()

	if os.Getenv(hubIntegrationRunEnv) != "1" {
		t.Skipf("skipping Hub integration tests: set %s=1 to enable", hubIntegrationRunEnv)
	}

	return requireIntegrationConfig(t)
}

func containsUserID(group *NestedGroup, userID string) bool {
	if group == nil {
		return false
	}
	for _, u := range group.Users {
		if u.ID == userID {
			return true
		}
	}
	for _, u := range group.OwnUsers {
		if u.ID == userID {
			return true
		}
	}

	return false
}

func isNotFoundOrMethodNotAllowed(err error) bool {
	if err == nil {
		return false
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}

	return httpErr.StatusCode == 404 || httpErr.StatusCode == 405
}

func integrationUserPassword(stamp string) string {
	if envPassword := strings.TrimSpace(os.Getenv(integrationUserPass)); envPassword != "" {
		return envPassword
	}

	// Default strong test password for instances that enforce local credentials.
	return "ItSync-" + stamp + "-Aa1!"
}

func resolveUserIDByLogin(ctx context.Context, client *Client, login string) (string, error) {
	var lastErr error

	for i := 0; i < 10; i++ {
		holder, err := client.GetUserByLogin(ctx, login)
		if err == nil && holder != nil && holder.Id != "" {
			return holder.Id, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("user id resolution returned empty result")
	}

	return "", lastErr
}

func createIntegrationUser(t *testing.T, ctx context.Context, client *Client, stamp string) *User {
	t.Helper()

	login := "it-sync-" + stamp
	createdUser, err := client.CreateUser(ctx, User{
		Login:    login,
		FullName: "Integration " + stamp,
		Email:    login + "@example.com",
		Password: integrationUserPassword(stamp),
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if createdUser.ID == "" {
		t.Fatal("created user has empty id")
	}

	canonicalID, err := resolveUserIDByLogin(ctx, client, createdUser.Login)
	if err != nil {
		t.Fatalf("failed to resolve canonical user id for login %s: %v", createdUser.Login, err)
	}
	createdUser.ID = canonicalID

	return createdUser
}

func updateIntegrationUser(t *testing.T, ctx context.Context, client *Client, user *User, stamp string) {
	t.Helper()

	updatedName := "Integration Updated " + stamp
	updatedUser, err := client.UpdateUser(ctx, user.ID, User{
		Login:    user.Login,
		Email:    user.Email,
		FullName: updatedName,
	})
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}
	if updatedUser.FullName != updatedName {
		t.Fatalf("unexpected updated user fullName: got %q, want %q", updatedUser.FullName, updatedName)
	}
}

func createIntegrationGroup(t *testing.T, ctx context.Context, client *Client, stamp string) *NestedGroup {
	t.Helper()

	groupName := "it-sync-group-" + stamp
	createdGroup, err := client.CreateGroup(ctx, NestedGroup{Name: groupName, Description: "integration test group"})
	if err != nil {
		t.Fatalf("failed to create group: %v", err)
	}
	if createdGroup.ID == "" {
		t.Fatal("created group has empty id")
	}

	return createdGroup
}

func updateIntegrationGroup(t *testing.T, ctx context.Context, client *Client, groupID string) {
	t.Helper()

	if _, err := client.UpdateGroup(ctx, groupID, NestedGroup{Description: "integration test group updated"}); err != nil {
		t.Fatalf("failed to update group: %v", err)
	}
}

func addUserAndAssertMembership(t *testing.T, ctx context.Context, client *Client, groupID, userRef, userID string) {
	t.Helper()

	if err := client.AddUserToGroup(ctx, groupID, userRef); err != nil {
		if isNotFoundOrMethodNotAllowed(err) {
			t.Skipf("skipping Hub membership test on this instance: add endpoint unavailable/incompatible: %v", err)
		}
		t.Fatalf("failed to add user to group: %v", err)
	}

	groupAfterAdd, err := client.GetGroupByID(ctx, groupID)
	if err != nil {
		t.Fatalf("failed to get group after add: %v", err)
	}
	if !containsUserID(groupAfterAdd, userID) {
		t.Fatalf("user %s not found in group %s after add", userID, groupID)
	}
}

func removeUserAndAssertMembershipGone(t *testing.T, ctx context.Context, client *Client, groupID, userRef, userID string) {
	t.Helper()

	if err := client.RemoveUserFromGroup(ctx, groupID, userRef); err != nil {
		if isNotFoundOrMethodNotAllowed(err) {
			t.Skipf("skipping Hub membership test on this instance: remove endpoint unavailable/incompatible: %v", err)
		}
		t.Fatalf("failed to remove user from group: %v", err)
	}

	groupAfterRemove, err := client.GetGroupByID(ctx, groupID)
	if err != nil {
		t.Fatalf("failed to get group after remove: %v", err)
	}
	if containsUserID(groupAfterRemove, userID) {
		t.Fatalf("user %s still present in group %s after remove", userID, groupID)
	}
}

func deleteUserOrFail(t *testing.T, ctx context.Context, client *Client, userID string) {
	t.Helper()

	if err := client.DeleteUser(ctx, userID); err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}
}

func deleteGroupOrFail(t *testing.T, ctx context.Context, client *Client, groupID string) {
	t.Helper()

	allUsers, err := client.GetAllUsersGroup(ctx)
	if err != nil {
		t.Fatalf("failed to get all users group before group delete: %v", err)
	}

	if err := client.DeleteGroup(ctx, groupID, allUsers.ID); err != nil {
		t.Fatalf("failed to delete group: %v", err)
	}
}

func registerCleanupUser(t *testing.T, client *Client, createdUser **User) {
	t.Helper()

	t.Cleanup(func() {
		if *createdUser != nil && (*createdUser).ID != "" {
			if cleanupErr := client.DeleteUser(context.Background(), (*createdUser).ID); cleanupErr != nil {
				t.Logf("cleanup warning: failed to delete user %s: %v", (*createdUser).ID, cleanupErr)
			}
		}
	})
}

func registerCleanupGroup(t *testing.T, client *Client, createdGroup **NestedGroup) {
	t.Helper()

	t.Cleanup(func() {
		if *createdGroup != nil && (*createdGroup).ID != "" {
			allUsers, allUsersErr := client.GetAllUsersGroup(context.Background())
			if allUsersErr != nil {
				t.Logf("cleanup warning: failed to get all users group: %v", allUsersErr)
				return
			}
			if cleanupErr := client.DeleteGroup(context.Background(), (*createdGroup).ID, allUsers.ID); cleanupErr != nil {
				t.Logf("cleanup warning: failed to delete group %s: %v", (*createdGroup).ID, cleanupErr)
			}
		}
	})
}

func TestIntegrationYouTrackGroupLifecycle(t *testing.T) {
	client, ctx := requireIntegrationConfig(t)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())

	createdGroup := createIntegrationGroup(t, ctx, client, stamp)
	registerCleanupGroup(t, client, &createdGroup)

	updateIntegrationGroup(t, ctx, client, createdGroup.ID)

	deleteGroupOrFail(t, ctx, client, createdGroup.ID)
	createdGroup = nil
}

func TestIntegrationHubUserGroupMembershipLifecycle(t *testing.T) {
	client, ctx := requireHubIntegrationConfig(t)

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())

	createdUser := createIntegrationUser(t, ctx, client, stamp)
	registerCleanupUser(t, client, &createdUser)

	updateIntegrationUser(t, ctx, client, createdUser, stamp)

	createdGroup := createIntegrationGroup(t, ctx, client, stamp)
	registerCleanupGroup(t, client, &createdGroup)

	updateIntegrationGroup(t, ctx, client, createdGroup.ID)

	membershipUserRef := createdUser.ID
	if createdUser.RingID != "" {
		membershipUserRef = createdUser.RingID
	}

	addUserAndAssertMembership(t, ctx, client, createdGroup.ID, membershipUserRef, createdUser.ID)
	removeUserAndAssertMembershipGone(t, ctx, client, createdGroup.ID, membershipUserRef, createdUser.ID)

	deleteUserOrFail(t, ctx, client, createdUser.ID)
	createdUser = nil

	deleteGroupOrFail(t, ctx, client, createdGroup.ID)
	createdGroup = nil
}
