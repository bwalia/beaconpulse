package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/monitor"
	"beacon/internal/domain/statuspage"
)

// StatusPageRepository implements statuspage.Repository.
type StatusPageRepository struct {
	pool *pgxpool.Pool
}

// NewStatusPageRepository builds a StatusPageRepository.
func NewStatusPageRepository(pool *pgxpool.Pool) *StatusPageRepository {
	return &StatusPageRepository{pool: pool}
}

var _ statuspage.Repository = (*StatusPageRepository)(nil)

// statusPageQuery loads every published monitor for a published org, in one pass.
//
// The WHERE clause IS the security boundary, so it is worth reading closely:
//
//	o.status_page_enabled  — the org opted in to having a public page at all
//	m.public               — this specific monitor was published onto it
//	m.enabled              — a paused monitor is not evidence of anything
//	*.deleted_at IS NULL   — deleted orgs/monitors never resurface publicly
//
// The SELECT list is equally deliberate: it names only the columns the public
// projection is allowed to carry. `m.target` is never read, so it cannot leak
// through a later refactor of the row scan.
//
// Ordering is stable (project, then monitor name) so the page does not reshuffle
// between refreshes — a status page that reorders itself looks broken.
const statusPageQuery = `
SELECT o.name,
       o.status_page_title,
       p.name,
       p.environment,
       m.name,
       m.last_status,
       m.last_checked_at
  FROM organizations o
  JOIN monitors      m ON m.org_id = o.id
  JOIN projects      p ON p.id = m.project_id
 WHERE o.slug = $1
   AND o.status_page_enabled
   AND o.deleted_at IS NULL
   AND m.public
   AND m.enabled
   AND m.deleted_at IS NULL
   AND p.deleted_at IS NULL
 ORDER BY p.name, m.name`

// GetBySlug returns the published page, or (nil, nil) when the slug is unknown OR
// the org has not published a page.
//
// Those two cases MUST be indistinguishable: returning "exists but not published"
// would turn this endpoint into an oracle for enumerating which organizations
// have accounts. Both return nil.
func (r *StatusPageRepository) GetBySlug(ctx context.Context, slug string) (*statuspage.Page, error) {
	rows, err := r.pool.Query(ctx, statusPageQuery, slug)
	if err != nil {
		return nil, fmt.Errorf("status page query: %w", err)
	}
	defer rows.Close()

	page := &statuspage.Page{UpdatedAt: time.Now().UTC()}
	// Preserve the SQL ordering while grouping: index by name, append in first-seen
	// order. A map alone would randomise the group order on every request.
	idx := make(map[string]int)

	for rows.Next() {
		var (
			orgName  string
			title    string
			projName string
			env      string
			monName  string
			status   string
			checked  *time.Time
		)
		if err := rows.Scan(&orgName, &title, &projName, &env, &monName, &status, &checked); err != nil {
			return nil, fmt.Errorf("status page scan: %w", err)
		}
		page.OrgName = orgName
		page.Title = title

		i, ok := idx[projName]
		if !ok {
			page.Groups = append(page.Groups, statuspage.Group{Name: projName, Environment: env})
			i = len(page.Groups) - 1
			idx[projName] = i
		}
		page.Groups[i].Monitors = append(page.Groups[i].Monitors, statuspage.Monitor{
			Name:          monName,
			Status:        monitor.Status(status),
			LastCheckedAt: checked,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("status page rows: %w", err)
	}

	// No rows means either the slug is unknown, the org has not published, or it
	// has published nothing yet. The first two must look identical from outside.
	// The third is genuinely empty — but it is also indistinguishable, and an
	// empty published page carries no information anyway, so collapsing all three
	// to "not found" is both safe and honest.
	if len(page.Groups) == 0 {
		return nil, nil
	}
	return page, nil
}
