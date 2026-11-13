package team

import (
	"context"
	"fmt"
	"sort"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"
)

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Person struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
}

type Plan struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	StartsOn     string `json:"starts_on"`
	CurrentStage string `json:"current_stage"`
}

type Progress struct {
	Done       int `json:"done"`
	Total      int `json:"total"`
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Skipped    int `json:"skipped"`
	Percent    int `json:"percent"`
}

type Pace struct {
	Expected  int `json:"expected"`
	Completed int `json:"completed"`
	Percent   int `json:"percent"`
}

type Blocker struct {
	ItemID string `json:"item_id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

type Overdue struct {
	ItemID      string `json:"item_id"`
	Title       string `json:"title"`
	DueOn       string `json:"due_on"`
	DaysOverdue int    `json:"days_overdue"`
}

type Activity struct {
	Questions   int        `json:"questions"`
	LastEventAt *time.Time `json:"last_event_at,omitempty"`
}

type Member struct {
	ID       string    `json:"id"`
	FullName string    `json:"full_name"`
	Email    string    `json:"email"`
	Position string    `json:"position"`
	Team     Team      `json:"team"`
	Mentor   *Person   `json:"mentor"`
	Plan     *Plan     `json:"plan"`
	Progress Progress  `json:"progress"`
	Pace     Pace      `json:"pace"`
	Blockers []Blocker `json:"blockers"`
	Overdue  []Overdue `json:"overdue"`
	Activity Activity  `json:"activity"`
	AtRisk   bool      `json:"at_risk"`
}

type TeamStat struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Interns     int    `json:"interns"`
	AvgProgress int    `json:"avg_progress"`
	AvgPace     int    `json:"avg_pace"`
	Blocked     int    `json:"blocked"`
	Overdue     int    `json:"overdue"`
}

type Overview struct {
	Interns     int        `json:"interns"`
	AvgProgress int        `json:"avg_progress"`
	AvgPace     int        `json:"avg_pace"`
	Blocked     int        `json:"blocked"`
	Overdue     int        `json:"overdue"`
	AtRisk      int        `json:"at_risk"`
	Teams       []TeamStat `json:"teams"`
}

type Report struct {
	Interns  []Member `json:"interns"`
	Overview Overview `json:"overview"`
}

type Filter struct {
	TeamID   string
	MentorID string
	All      bool
}

type InternRecord struct {
	ID         string
	FullName   string
	Email      string
	Position   string
	TeamID     string
	TeamName   string
	TeamSlug   string
	MentorID   string
	MentorName string
	PlanID     string
	PlanTitle  string
	StartsOn   time.Time
	HasPlan    bool
}

type ItemRecord struct {
	InternID      string
	ItemID        string
	StageTitle    string
	StagePosition int
	Position      int
	Title         string
	Status        string
	BlockedReason string
	DueOn         *time.Time
}

type ActivityRecord struct {
	UserID      string
	Questions   int
	LastEventAt *time.Time
}

type Snapshot struct {
	Interns  []InternRecord
	Items    []ItemRecord
	Activity map[string]ActivityRecord
}

type Store interface {
	Snapshot(ctx context.Context, filter Filter) (Snapshot, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Report(ctx context.Context, principal reqctx.Principal) (Report, error) {
	if s == nil || s.store == nil {
		return Report{}, fmt.Errorf("team: store is not initialised")
	}

	snapshot, err := s.store.Snapshot(ctx, filterFor(principal))
	if err != nil {
		return Report{}, err
	}

	members := buildMembers(snapshot, s.now())

	return Report{Interns: members, Overview: buildOverview(members)}, nil
}

func filterFor(principal reqctx.Principal) Filter {
	all := rbac.HasScope(principal.Scopes, rbac.ScopeAll) || principal.Role == rbac.RoleHR

	return Filter{TeamID: principal.TeamID, MentorID: principal.UserID, All: all}
}

func buildMembers(snapshot Snapshot, now time.Time) []Member {
	today := now.Truncate(24 * time.Hour)
	itemsByIntern := map[string][]ItemRecord{}

	for _, item := range snapshot.Items {
		itemsByIntern[item.InternID] = append(itemsByIntern[item.InternID], item)
	}

	members := make([]Member, 0, len(snapshot.Interns))

	for _, intern := range snapshot.Interns {
		items := itemsByIntern[intern.ID]

		sort.SliceStable(items, func(i, j int) bool {
			if items[i].StagePosition != items[j].StagePosition {
				return items[i].StagePosition < items[j].StagePosition
			}
			return items[i].Position < items[j].Position
		})

		member := Member{
			ID:       intern.ID,
			FullName: intern.FullName,
			Email:    intern.Email,
			Position: intern.Position,
			Team: Team{
				ID:   intern.TeamID,
				Name: intern.TeamName,
				Slug: intern.TeamSlug,
			},
			Blockers: []Blocker{},
			Overdue:  []Overdue{},
		}

		if intern.MentorID != "" {
			member.Mentor = &Person{ID: intern.MentorID, FullName: intern.MentorName}
		}

		if intern.HasPlan {
			member.Plan = &Plan{
				ID:           intern.PlanID,
				Title:        intern.PlanTitle,
				StartsOn:     intern.StartsOn.Format(time.DateOnly),
				CurrentStage: currentStageOf(items),
			}
		}

		member.Progress = progressOf(items)
		member.Pace = paceOf(items, today, member.Progress)
		member.Blockers = blockersOf(items)
		member.Overdue = overdueOf(items, today)
		member.Activity = activityOf(snapshot.Activity[intern.ID])
		member.AtRisk = len(member.Blockers) > 0 || len(member.Overdue) > 0

		members = append(members, member)
	}

	return members
}

func progressOf(items []ItemRecord) Progress {
	progress := Progress{Total: len(items)}

	for _, item := range items {
		switch item.Status {
		case "done":
			progress.Done++
		case "in_progress":
			progress.InProgress++
		case "blocked":
			progress.Blocked++
		case "skipped":
			progress.Skipped++
		default:
			progress.Pending++
		}
	}

	if progress.Total > 0 {
		progress.Percent = progress.Done * 100 / progress.Total
	}

	return progress
}

func paceOf(items []ItemRecord, today time.Time, progress Progress) Pace {
	pace := Pace{}

	for _, item := range items {
		if item.DueOn == nil {
			continue
		}

		due := item.DueOn.Truncate(24 * time.Hour)
		if due.After(today) {
			continue
		}

		pace.Expected++

		if item.Status == "done" {
			pace.Completed++
		}
	}

	switch {
	case pace.Expected > 0:
		pace.Percent = pace.Completed * 100 / pace.Expected
	case progress.Total > 0:
		pace.Percent = progress.Percent
	}

	return pace
}

func blockersOf(items []ItemRecord) []Blocker {
	blockers := []Blocker{}

	for _, item := range items {
		if item.Status != "blocked" {
			continue
		}

		blockers = append(blockers, Blocker{
			ItemID: item.ItemID,
			Title:  item.Title,
			Reason: item.BlockedReason,
		})
	}

	return blockers
}

func overdueOf(items []ItemRecord, today time.Time) []Overdue {
	overdue := []Overdue{}

	for _, item := range items {
		if item.DueOn == nil {
			continue
		}

		if item.Status == "done" || item.Status == "skipped" {
			continue
		}

		due := item.DueOn.Truncate(24 * time.Hour)
		if !due.Before(today) {
			continue
		}

		overdue = append(overdue, Overdue{
			ItemID:      item.ItemID,
			Title:       item.Title,
			DueOn:       item.DueOn.Format(time.DateOnly),
			DaysOverdue: int(today.Sub(due).Hours() / 24),
		})
	}

	return overdue
}

func activityOf(record ActivityRecord) Activity {
	activity := Activity{Questions: record.Questions}

	if record.LastEventAt != nil {
		last := *record.LastEventAt
		activity.LastEventAt = &last
	}

	return activity
}

func currentStageOf(items []ItemRecord) string {
	current := ""

	for _, item := range items {
		if item.Status == "done" || item.Status == "skipped" {
			continue
		}

		return item.StageTitle
	}

	if len(items) > 0 {
		current = items[len(items)-1].StageTitle
	}

	return current
}

func buildOverview(members []Member) Overview {
	overview := Overview{Interns: len(members), Teams: []TeamStat{}}

	if len(members) == 0 {
		return overview
	}

	progressSum := 0
	paceSum := 0
	teams := map[string]*TeamStat{}
	order := []string{}

	for _, member := range members {
		progressSum += member.Progress.Percent
		paceSum += member.Pace.Percent
		overview.Blocked += member.Progress.Blocked
		overview.Overdue += len(member.Overdue)

		if member.AtRisk {
			overview.AtRisk++
		}

		key := member.Team.ID
		if key == "" {
			key = "no-team"
		}

		stat, ok := teams[key]
		if !ok {
			stat = &TeamStat{ID: member.Team.ID, Name: member.Team.Name}
			teams[key] = stat
			order = append(order, key)
		}

		stat.Interns++
		stat.AvgProgress += member.Progress.Percent
		stat.AvgPace += member.Pace.Percent
		stat.Blocked += member.Progress.Blocked
		stat.Overdue += len(member.Overdue)
	}

	overview.AvgProgress = progressSum / len(members)
	overview.AvgPace = paceSum / len(members)

	for _, key := range order {
		stat := teams[key]
		if stat.Interns > 0 {
			stat.AvgProgress /= stat.Interns
			stat.AvgPace /= stat.Interns
		}
		overview.Teams = append(overview.Teams, *stat)
	}

	return overview
}
