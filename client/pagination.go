package youtrack

import (
	"context"
	"fmt"
)

const (
	// defaultPageSize is the page size used by the ListAll* methods. YouTrack
	// applies its own default when $top is omitted, which differs per endpoint,
	// so paging to exhaustion has to set an explicit size to know when the last
	// page has been reached.
	defaultPageSize = 100

	// maxPagesSafetyLimit bounds a ListAll* walk. A server that ignores $skip
	// would otherwise return the same full page forever and the call would
	// never return, hanging a reconcile until its context expires. At the
	// default page size this allows 100000 entities, far more than any of these
	// admin collections holds.
	maxPagesSafetyLimit = 1000
)

// listAll pages through a list endpoint until it is exhausted and returns every
// entity.
//
// Reconciling callers need the complete collection to decide what to create,
// update, and delete; reading only the first page makes a controller converge
// toward deleting everything it could not see. Writing that loop at each call
// site invites exactly that bug, so it lives here once.
//
// A page shorter than pageSize ends the walk, which is how YouTrack signals the
// last page.
func listAll[T any](ctx context.Context, pageSize int, fetchPage func(context.Context, int, int) ([]T, error)) ([]T, error) {
	var all []T

	for page := 0; page < maxPagesSafetyLimit; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		items, err := fetchPage(ctx, pageSize, page*pageSize)
		if err != nil {
			return nil, err
		}

		all = append(all, items...)

		if len(items) < pageSize {
			return all, nil
		}
	}

	return nil, fmt.Errorf("youtrack: list did not terminate after %d pages of %d", maxPagesSafetyLimit, pageSize)
}

// ListAllApps returns every installed app, paging until the list is exhausted.
// Prefer it over ListApps when the caller needs the complete set, such as a
// reconcile that must not act on a partial view.
func (c *Client) ListAllApps(ctx context.Context) ([]App, error) {
	return listAll(ctx, defaultPageSize, c.ListApps)
}

// ListAllProjects returns every project, paging until the list is exhausted.
func (c *Client) ListAllProjects(ctx context.Context) ([]Project, error) {
	return listAll(ctx, defaultPageSize, c.ListProjects)
}

// ListAllYoutrackRoles returns every role, paging until the list is exhausted.
func (c *Client) ListAllYoutrackRoles(ctx context.Context) ([]Role, error) {
	return listAll(ctx, defaultPageSize, c.ListYoutrackRoles)
}

// ListAllAssignedRoles returns every role assignment, paging until the list is
// exhausted.
func (c *Client) ListAllAssignedRoles(ctx context.Context) ([]AssignedRole, error) {
	return listAll(ctx, defaultPageSize, c.GetAllAssignedRoles)
}

// ListAllServices returns every Hub service, paging until the list is
// exhausted.
func (c *Client) ListAllServices(ctx context.Context) ([]Service, error) {
	return listAll(ctx, defaultPageSize, c.ListServices)
}

// ListAllUsers returns every user, paging until the list is exhausted.
func (c *Client) ListAllUsers(ctx context.Context) ([]Holder, error) {
	return listAll(ctx, defaultPageSize, c.ListUsers)
}

// ListAllGroups returns every group, paging until the list is exhausted.
func (c *Client) ListAllGroups(ctx context.Context) ([]Holder, error) {
	return listAll(ctx, defaultPageSize, c.ListGroups)
}
