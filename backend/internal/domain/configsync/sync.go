// Package configsync applies a declared set of monitors to an organization.
//
// It exists because CI is not a person. A workflow that POSTs its monitors on every
// push would create a duplicate on every re-run, retry and re-merge, and a fortnight
// later the same domain is being probed forty times and billed forty times. The fix is
// not to ask the workflow to remember what it created — it cannot, reliably — but to
// change the question from "add this" to "make it so": here is the desired set, work
// out the difference. That is safe to run a thousand times, which is the only property
// that matters when a machine is doing the running.
//
// Everything goes through the monitor and project SERVICES rather than their
// repositories, so a synced monitor is subject to exactly the same plan limits,
// validation and control-plane push as one created by hand. A sync path that reached
// past those would be a way to buy 500 monitors on a Free plan.
package configsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/domain/monitor"
	"beacon/internal/domain/project"
	"beacon/internal/platform/apperror"
)

// Actor is the authenticated caller.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}

// DesiredMonitor is one entry from the caller's declared set. It mirrors what a
// monitor can be configured with, so a file in a repository can express anything the
// dashboard can.
type DesiredMonitor struct {
	Name            string
	Type            string
	Target          string
	IntervalSeconds int
	TimeoutSeconds  int
	GraceSeconds    int
	Enabled         *bool
	Public          *bool
	Settings        monitor.Settings
}

// Input is a whole declaration.
type Input struct {
	// Project names the group these monitors belong to, created if it does not exist
	// so a first run needs no setup. Empty means the org's default project.
	Project string
	Monitors []DesiredMonitor
	// Prune removes monitors in the project that the declaration no longer mentions.
	//
	// OFF by default, and that default is the safety property. A workflow with a bad
	// path glob, an empty matrix, or a failed template step declares zero monitors —
	// and with pruning on by default that silently deletes production monitoring at
	// the exact moment nobody is watching. Off, the same mistake reports what it would
	// have removed and changes nothing.
	Prune bool
	// DryRun computes the plan and applies none of it. Meant for pull requests: a
	// workflow can comment "this will add 2 and remove 1" before anyone merges.
	DryRun bool
}

// ItemResult is what happened to one declared monitor.
type ItemResult struct {
	Name   string `json:"name"`
	Action string `json:"action"` // created | updated | unchanged | removed | would_remove | error
	ID     string `json:"id,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Result is the whole outcome. Counts are alongside the items because CI reads the
// counts and humans read the items.
type Result struct {
	Project     string       `json:"project"`
	DryRun      bool         `json:"dry_run"`
	Created     int          `json:"created"`
	Updated     int          `json:"updated"`
	Unchanged   int          `json:"unchanged"`
	Removed     int          `json:"removed"`
	WouldRemove int          `json:"would_remove"`
	Failed      int          `json:"failed"`
	Items       []ItemResult `json:"items"`
}

// Monitors is the slice of the monitor service this package needs.
type Monitors interface {
	List(ctx context.Context, actor monitor.Actor, f monitor.ListFilter) ([]monitor.Monitor, int, error)
	Create(ctx context.Context, actor monitor.Actor, in monitor.CreateInput) (*monitor.Monitor, error)
	Update(ctx context.Context, actor monitor.Actor, id uuid.UUID, in monitor.UpdateInput) (*monitor.Monitor, error)
	Delete(ctx context.Context, actor monitor.Actor, id uuid.UUID) error
}

// Projects is the slice of the project service this package needs.
type Projects interface {
	List(ctx context.Context, actor project.Actor, f project.ListFilter) ([]project.Project, int, error)
	Create(ctx context.Context, actor project.Actor, in project.CreateInput) (*project.Project, error)
}

// Service applies declarations.
type Service struct {
	monitors Monitors
	projects Projects
}

func NewService(monitors Monitors, projects Projects) *Service {
	return &Service{monitors: monitors, projects: projects}
}

// maxDeclared bounds one request. Generous for real use — nobody hand-maintains more
// than a few hundred domains in a file — and it stops a malformed generator turning
// one request into thousands of writes.
const maxDeclared = 500

// Apply reconciles the declared set against what exists.
//
// Monitors are matched by NAME within the project, because a name is the only
// identifier a file in a repository has: ids are assigned by us, so requiring one
// would mean the workflow storing state it has no good place to keep.
func (s *Service) Apply(ctx context.Context, actor Actor, in Input) (*Result, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit changing monitors")
	}
	if len(in.Monitors) > maxDeclared {
		return nil, apperror.Validation("too many monitors in one request",
			apperror.FieldError{
				Field:   "monitors",
				Message: fmt.Sprintf("declare at most %d at a time", maxDeclared),
			})
	}
	if err := validateNames(in.Monitors); err != nil {
		return nil, err
	}

	mActor := monitor.Actor{UserID: actor.UserID, OrgID: actor.OrgID, Role: actor.Role}
	proj, err := s.resolveProject(ctx, actor, in.Project, in.DryRun)
	if err != nil {
		return nil, err
	}

	res := &Result{Project: in.Project, DryRun: in.DryRun, Items: []ItemResult{}}

	// A dry run against a project that does not exist yet has nothing to compare to,
	// and reporting everything as "created" is exactly right: that is what applying
	// would do.
	existing := map[string]monitor.Monitor{}
	if proj != nil {
		id := proj.ID
		res.Project = proj.Name
		current, _, err := s.monitors.List(ctx, mActor, monitor.ListFilter{ProjectID: &id, Limit: maxDeclared * 2})
		if err != nil {
			return nil, err
		}
		for _, m := range current {
			existing[strings.ToLower(m.Name)] = m
		}
	}

	declared := map[string]bool{}
	for _, d := range in.Monitors {
		key := strings.ToLower(strings.TrimSpace(d.Name))
		declared[key] = true

		cur, found := existing[key]
		switch {
		case !found:
			res.Items = append(res.Items, s.create(ctx, mActor, proj, d, in.DryRun))
		case !changed(cur, d):
			// Reported rather than silently skipped: "unchanged" is how a workflow
			// log shows that a re-run did nothing, which is the whole promise.
			res.Items = append(res.Items, ItemResult{Name: d.Name, Action: "unchanged", ID: cur.ID.String()})
		default:
			res.Items = append(res.Items, s.update(ctx, mActor, cur, d, in.DryRun))
		}
	}

	// Anything present but no longer declared. Reported by default, removed only when
	// explicitly asked.
	for key, m := range existing {
		if declared[key] {
			continue
		}
		if !in.Prune {
			res.Items = append(res.Items, ItemResult{Name: m.Name, Action: "would_remove", ID: m.ID.String()})
			continue
		}
		if in.DryRun {
			res.Items = append(res.Items, ItemResult{Name: m.Name, Action: "would_remove", ID: m.ID.String()})
			continue
		}
		if err := s.monitors.Delete(ctx, mActor, m.ID); err != nil {
			res.Items = append(res.Items, ItemResult{Name: m.Name, Action: "error", ID: m.ID.String(), Error: message(err)})
			continue
		}
		res.Items = append(res.Items, ItemResult{Name: m.Name, Action: "removed", ID: m.ID.String()})
	}

	// Sorted, because the removal pass iterates a map and Go randomises that order.
	// Two identical runs would otherwise return the same items shuffled, and the first
	// thing anyone does with CI output is diff it against the last run.
	sort.Slice(res.Items, func(i, j int) bool { return res.Items[i].Name < res.Items[j].Name })

	tally(res)
	return res, nil
}

// create adds one monitor, reporting failure per item rather than aborting.
//
// One rejected entry — a typo'd target, or the plan's monitor limit reached — must not
// discard the other ninety-nine that were fine. A workflow can then fix that one line;
// an all-or-nothing sync makes the whole file unappliable until it is perfect.
func (s *Service) create(ctx context.Context, actor monitor.Actor, proj *project.Project, d DesiredMonitor, dry bool) ItemResult {
	if dry {
		return ItemResult{Name: d.Name, Action: "created"}
	}
	if proj == nil {
		return ItemResult{Name: d.Name, Action: "error", Error: "project could not be resolved"}
	}
	m, err := s.monitors.Create(ctx, actor, monitor.CreateInput{
		ProjectID:       proj.ID,
		Name:            d.Name,
		Type:            monitor.Type(d.Type),
		Target:          d.Target,
		Enabled:         d.Enabled,
		Public:          d.Public,
		IntervalSeconds: d.IntervalSeconds,
		TimeoutSeconds:  d.TimeoutSeconds,
		GraceSeconds:    d.GraceSeconds,
		Settings:        d.Settings,
	})
	if err != nil {
		return ItemResult{Name: d.Name, Action: "error", Error: message(err)}
	}
	return ItemResult{Name: d.Name, Action: "created", ID: m.ID.String()}
}

func (s *Service) update(ctx context.Context, actor monitor.Actor, cur monitor.Monitor, d DesiredMonitor, dry bool) ItemResult {
	if dry {
		return ItemResult{Name: d.Name, Action: "updated", ID: cur.ID.String()}
	}
	target := d.Target
	settings := d.Settings
	in := monitor.UpdateInput{Target: &target, Settings: &settings}
	if d.IntervalSeconds > 0 {
		iv := d.IntervalSeconds
		in.IntervalSeconds = &iv
	}
	if d.Enabled != nil {
		in.Enabled = d.Enabled
	}
	if d.Public != nil {
		in.Public = d.Public
	}
	m, err := s.monitors.Update(ctx, actor, cur.ID, in)
	if err != nil {
		return ItemResult{Name: d.Name, Action: "error", ID: cur.ID.String(), Error: message(err)}
	}
	return ItemResult{Name: d.Name, Action: "updated", ID: m.ID.String()}
}

// resolveProject finds the named project or creates it. A first run should not require
// the user to have clicked anything in the dashboard first.
func (s *Service) resolveProject(ctx context.Context, actor Actor, name string, dry bool) (*project.Project, error) {
	pActor := project.Actor{UserID: actor.UserID, OrgID: actor.OrgID, Role: actor.Role}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Default"
	}

	list, _, err := s.projects.List(ctx, pActor, project.ListFilter{Limit: 200})
	if err != nil {
		return nil, err
	}
	for i := range list {
		if strings.EqualFold(list[i].Name, name) {
			return &list[i], nil
		}
	}
	if dry {
		return nil, nil // nothing exists yet; the plan is "everything is new"
	}
	return s.projects.Create(ctx, pActor, project.CreateInput{Name: name})
}

// changed reports whether the declaration differs from what is stored, so a re-run
// with no edits performs no writes at all. Without this every sync would rewrite every
// monitor and trigger a control-plane reload for nothing.
func changed(cur monitor.Monitor, d DesiredMonitor) bool {
	if !strings.EqualFold(cur.Target, d.Target) {
		return true
	}
	if d.IntervalSeconds > 0 && cur.IntervalSeconds != d.IntervalSeconds {
		return true
	}
	if d.Enabled != nil && cur.Enabled != *d.Enabled {
		return true
	}
	if d.Public != nil && cur.Public != *d.Public {
		return true
	}
	if !sameSettings(cur.Settings, d.Settings) {
		return true
	}
	return false
}

// sameSettings compares settings by their JSON form.
//
// Settings holds a map and a slice, so == does not compile and a field-by-field
// comparison would silently stop covering any field added later — which would show up
// as a sync that reports "unchanged" while quietly ignoring an edit. Marshalling
// compares whatever the struct currently is, and Go sorts map keys when encoding, so
// the result is stable rather than order-dependent.
func sameSettings(a, b monitor.Settings) bool {
	ja, erra := json.Marshal(a)
	jb, errb := json.Marshal(b)
	if erra != nil || errb != nil {
		return false // unencodable: treat as different and let the update decide
	}
	return bytes.Equal(ja, jb)
}

func validateNames(items []DesiredMonitor) error {
	seen := map[string]bool{}
	for _, d := range items {
		name := strings.ToLower(strings.TrimSpace(d.Name))
		if name == "" {
			return apperror.Validation("every monitor needs a name",
				apperror.FieldError{Field: "monitors[].name", Message: "the name is how a monitor is matched across runs"})
		}
		// Duplicates within one declaration are rejected rather than resolved: the two
		// entries would fight on every run, one overwriting the other, and the sync
		// would report changes forever while converging on nothing.
		if seen[name] {
			return apperror.Validation("duplicate monitor name in this request",
				apperror.FieldError{Field: "monitors[].name", Message: fmt.Sprintf("%q appears more than once", d.Name)})
		}
		seen[name] = true
	}
	return nil
}

func tally(r *Result) {
	for _, it := range r.Items {
		switch it.Action {
		case "created":
			r.Created++
		case "updated":
			r.Updated++
		case "unchanged":
			r.Unchanged++
		case "removed":
			r.Removed++
		case "would_remove":
			r.WouldRemove++
		case "error":
			r.Failed++
		}
	}
}

// message renders an error for a per-item report. Validation and limit errors are the
// useful ones — "monitor limit reached on the free plan" is the whole answer — and
// anything internal is deliberately not echoed into a CI log.
func message(err error) string {
	ae := apperror.From(err)
	switch ae.Code {
	case apperror.CodeInternal:
		return "internal error"
	default:
		return ae.Message
	}
}
