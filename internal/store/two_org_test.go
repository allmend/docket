package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/allmend/docket/internal/model"
)

// orgWorld is one org with a bit of everything: a workspace, a board with an
// active sprint, tickets in and out of it, a track, a comment, history, a
// blocking link, an assignee, a DoD item, a retro card and an API token. Every
// name in it carries the org's marker, so a leak shows up as text.
type orgWorld struct {
	marker            string
	org               *model.Org
	user              *model.User
	team              *model.Team
	board             *model.Board
	col               *model.Column
	sprint            *model.Sprint
	inSprint, backlog *model.Ticket
	tag               *model.Tag
	comment           *model.Comment
	link              *model.TicketLink
	retro             *model.RetroBoard
	card              *model.RetroCard
	token             *model.APIToken
}

func buildOrgWorld(t *testing.T, marker string) *orgWorld {
	t.Helper()
	ctx := context.Background()
	st := testStore
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: build %s: %v", marker, what, err)
		}
	}
	w := &orgWorld{marker: marker}
	var err error
	lower := strings.ToLower(marker)

	w.org, err = st.CreateOrg(ctx, marker+" org", lower)
	must(err, "org")
	w.user, err = st.CreateUser(ctx, w.org.ID, lower+"-user", marker+" User", lower+"@example.com", "admin")
	must(err, "user")
	w.team, err = st.CreateTeam(ctx, w.org.ID, w.user.ID, marker+" Team", marker[:3], lower+"-team", marker+" description")
	must(err, "team")
	must(st.AddTeamMember(ctx, w.org.ID, w.team.ID, w.user.ID), "team member")
	w.board, err = st.CreateBoard(ctx, w.org.ID, w.user.ID, &w.team.ID, marker+" Board", "", model.BoardModeScrum)
	must(err, "board")
	w.col, err = st.CreateColumn(ctx, w.org.ID, w.board.ID, marker+" To Do", 1000)
	must(err, "column")
	_, err = st.CreateColumn(ctx, w.org.ID, w.board.ID, "Done", 2000)
	must(err, "done column")

	start, end := time.Now().Add(-24*time.Hour), time.Now().Add(13*24*time.Hour)
	w.sprint, err = st.CreateSprint(ctx, w.org.ID, w.board.ID, w.user.ID, marker+" Sprint", marker+" goal", &start, &end)
	must(err, "sprint")
	_, err = st.SetSprintStatus(ctx, w.org.ID, w.sprint.ID, model.SprintStatusActive)
	must(err, "start sprint")

	w.inSprint, err = st.CreateTicket(ctx, w.org.ID, w.board.ID, w.col.ID, w.user.ID, &w.team.ID, 1, marker+" sprint ticket", marker+" body", model.PriorityHigh, 1000)
	must(err, "sprint ticket")
	must(st.AssignTicketToSprint(ctx, w.org.ID, w.inSprint.ID, &w.sprint.ID), "assign to sprint")
	w.backlog, err = st.CreateTicket(ctx, w.org.ID, w.board.ID, w.col.ID, w.user.ID, &w.team.ID, 2, marker+" backlog ticket", "", model.PriorityLow, 2000)
	must(err, "backlog ticket")

	w.tag, err = st.CreateTag(ctx, w.org.ID, w.board.ID, marker+" Track", "#10b981", "", nil)
	must(err, "tag")
	must(st.AddTagToTicket(ctx, w.org.ID, w.inSprint.ID, w.tag.ID), "tag ticket")
	must(st.AddTicketAssignee(ctx, w.org.ID, w.inSprint.ID, w.user.ID), "assignee")
	w.comment, err = st.CreateComment(ctx, w.org.ID, w.inSprint.ID, w.user.ID, marker+" comment")
	must(err, "comment")
	must(st.AppendHistory(ctx, w.org.ID, w.inSprint.ID, w.user.ID, marker+" User", "title", "old", marker+" new"), "history")
	w.link, _, err = st.CreateLink(ctx, w.org.ID, w.backlog.ID, w.inSprint.ID, model.RelationBlocks)
	must(err, "link")
	_, err = st.CreateDodItem(ctx, w.org.ID, w.board.ID, marker+" DoD")
	must(err, "dod")
	must(st.CreateNotification(ctx, w.org.ID, w.user.ID, &w.inSprint.ID, &w.user.ID, marker+" User", "assigned"), "notification")

	w.retro, err = st.GetOrCreateRetroBoard(ctx, w.org.ID, w.board.ID, &w.sprint.ID)
	must(err, "retro board")
	w.card, err = st.CreateRetroCard(ctx, w.org.ID, w.retro.ID, w.user.ID, model.RetroWentWell, marker+" retro card")
	must(err, "retro card")
	w.token, err = st.CreateAPIToken(ctx, w.org.ID, w.user.ID, marker+" token", lower+"-token-hash", model.ScopeAPIRead)
	must(err, "token")
	return w
}

// ids lists every identifier in the world, for scanning results for leaks.
func (w *orgWorld) ids() []string {
	out := []string{w.marker, w.org.ID.String(), w.user.ID.String(), w.team.ID.String(),
		w.board.ID.String(), w.col.ID.String(), w.sprint.ID.String(), w.inSprint.ID.String(),
		w.backlog.ID.String(), w.tag.ID.String(), w.comment.ID.String(), w.link.ID.String(),
		w.retro.ID.String(), w.card.ID.String(), w.token.ID.String()}
	return out
}

// Multi-tenancy has so far been proven by reading code, which is how every
// cross-org bug was found. This runs it: two orgs with identical shapes, and
// every read path called as org A — with A's own IDs (B's rows must not join
// in) and with B's IDs (nothing may come back). Any of B's IDs or names in a
// result fails the test.
func TestTwoOrgs_ReadPathsNeverCrossOrgs(t *testing.T) {
	st := requireStore(t)
	ctx := context.Background()
	a := buildOrgWorld(t, "ALPHAORG")
	b := buildOrgWorld(t, "BRAVOORG")
	org := a.org.ID

	check := func(name string, v any, err error) {
		t.Helper()
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("%s: unexpected error %v", name, err)
			return
		}
		raw, jerr := json.Marshal(v)
		if jerr != nil {
			t.Fatalf("%s: marshal: %v", name, jerr)
		}
		got := string(raw)
		for _, id := range b.ids() {
			if strings.Contains(got, id) {
				t.Errorf("%s leaks org B (%s): %s", name, id, truncate(got, 300))
				return
			}
		}
	}

	// Org-wide reads, as A.
	{
		v, err := st.ListTeams(ctx, org)
		check("ListTeams", v, err)
		v2, err := st.ListTeamsWithBoards(ctx, org)
		check("ListTeamsWithBoards", v2, err)
		v3, err := st.ListOrgTags(ctx, org)
		check("ListOrgTags", v3, err)
		v4, err := st.ListBlockedTickets(ctx, org)
		check("ListBlockedTickets", v4, err)
		v5, err := st.ListRecentOrgActivity(ctx, org, 50)
		check("ListRecentOrgActivity", v5, err)
		v6, err := st.ListAPITokens(ctx, org)
		check("ListAPITokens", v6, err)
		v7, err := st.ListOrgMembers(ctx, org)
		check("ListOrgMembers", v7, err)
		v8, err := st.ListUsers(ctx, org)
		check("ListUsers", v8, err)
		v9, err := st.ListTeamMembersForOrg(ctx, org)
		check("ListTeamMembersForOrg", v9, err)
		for _, q := range []string{"ORG", "ticket", b.marker, "BRA-1"} {
			s1, err := st.SearchTickets(ctx, org, q, 50)
			check("SearchTickets "+q, s1, err)
			s2, err := st.SearchTicketsForMention(ctx, org, q)
			check("SearchTicketsForMention "+q, s2, err)
			s3, err := st.SearchTicketsForLink(ctx, org, a.backlog.ID, q)
			check("SearchTicketsForLink "+q, s3, err)
			s4, err := st.SearchUsers(ctx, org, q)
			check("SearchUsers "+q, s4, err)
			s5, err := st.SearchNonMembers(ctx, org, a.team.ID, q)
			check("SearchNonMembers "+q, s5, err)
		}
		_, err = st.GetTeamBySlug(ctx, org, b.team.Slug)
		check("GetTeamBySlug(B)", nil, err)
		tm, err := st.GetTeamByKey(ctx, org, b.team.Key)
		check("GetTeamByKey(B)", tm, err)
		tk, err := st.GetTicketByTeamRef(ctx, org, b.team.Key, 1)
		check("GetTicketByTeamRef(B)", tk, err)
		u, err := st.GetUserByUsername(ctx, org, b.user.Username)
		check("GetUserByUsername(B)", u, err)
	}

	// Per-entity reads, as A, first with A's IDs and then with B's.
	for _, w := range []*orgWorld{a, b} {
		p := "[" + w.marker + " ids] "
		r := func(name string, v any, err error) { t.Helper(); check(p+name, v, err) }

		v1, err := st.GetBoard(ctx, org, w.board.ID)
		r("GetBoard", v1, err)
		v2, err := st.GetBoardByTeamID(ctx, org, w.team.ID)
		r("GetBoardByTeamID", v2, err)
		v3, err := st.ListColumns(ctx, org, w.board.ID)
		r("ListColumns", v3, err)
		v4, err := st.GetColumn(ctx, org, w.col.ID)
		r("GetColumn", v4, err)
		v5, err := st.CountColumnTickets(ctx, org, w.col.ID)
		r("CountColumnTickets", v5, err)
		if w == b && v5 != (ColumnTicketCounts{}) {
			t.Errorf("CountColumnTickets counts org B's tickets: %+v", v5)
		}
		v6, err := st.ListTags(ctx, org, w.board.ID)
		r("ListTags", v6, err)
		v7, err := st.ListTrackStats(ctx, org, w.board.ID)
		r("ListTrackStats", v7, err)
		v8, err := st.ListTicketTags(ctx, org, w.inSprint.ID)
		r("ListTicketTags", v8, err)
		v9, err := st.BulkListTicketTags(ctx, org, w.board.ID)
		r("BulkListTicketTags", v9, err)
		v10, err := st.BulkListTicketTagsByTeam(ctx, org, w.team.ID)
		r("BulkListTicketTagsByTeam", v10, err)
		v11, err := st.GetTag(ctx, org, w.tag.ID)
		r("GetTag", v11, err)
		v12, err := st.ListTicketsByTag(ctx, org, w.tag.ID)
		r("ListTicketsByTag", v12, err)
		v13, err := st.ListSprints(ctx, org, w.board.ID)
		r("ListSprints", v13, err)
		v14, err := st.GetActiveSprint(ctx, org, w.board.ID)
		r("GetActiveSprint", v14, err)
		v15, err := st.GetSprint(ctx, org, w.sprint.ID)
		r("GetSprint", v15, err)
		v16, err := st.CountBacklogTickets(ctx, org, w.board.ID)
		r("CountBacklogTickets", v16, err)
		if w == b && v16 != 0 {
			t.Errorf("CountBacklogTickets counts org B's backlog: %d", v16)
		}
		v17, err := st.ListBacklogTickets(ctx, org, w.board.ID)
		r("ListBacklogTickets", v17, err)
		v18, err := st.ListSprintTickets(ctx, org, w.sprint.ID)
		r("ListSprintTickets", v18, err)
		done, returning, err := st.ListSprintTicketsForReview(ctx, org, w.sprint.ID)
		r("ListSprintTicketsForReview", []any{done, returning}, err)
		v19, err := st.ListBoardSprintTicketsSummary(ctx, org, w.board.ID)
		r("ListBoardSprintTicketsSummary", v19, err)
		v20, err := st.GetSprintCapacity(ctx, org, w.sprint.ID)
		r("GetSprintCapacity", v20, err)
		v21, err := st.ListDodItems(ctx, org, w.board.ID)
		r("ListDodItems", v21, err)
		v22, err := st.GetTicketDod(ctx, org, w.board.ID, w.inSprint.ID)
		r("GetTicketDod", v22, err)
		v23, err := st.ListMyOpenTickets(ctx, org, w.user.ID)
		r("ListMyOpenTickets", v23, err)
		v24, err := st.ListBlockedTicketsByTeam(ctx, org, w.team.ID)
		r("ListBlockedTicketsByTeam", v24, err)
		v25, err := st.ListMyOpenTicketsByTeam(ctx, org, w.user.ID, w.team.ID)
		r("ListMyOpenTicketsByTeam", v25, err)
		v26, err := st.ListRecentTeamActivity(ctx, org, w.team.ID, 50)
		r("ListRecentTeamActivity", v26, err)
		v27, err := st.ListComments(ctx, org, w.inSprint.ID)
		r("ListComments", v27, err)
		v28, err := st.GetComment(ctx, org, w.comment.ID)
		r("GetComment", v28, err)
		v29, err := st.ListHistory(ctx, org, w.inSprint.ID)
		r("ListHistory", v29, err)
		v30, err := st.ListNotificationsForUser(ctx, org, w.user.ID, 50)
		r("ListNotificationsForUser", v30, err)
		v31, err := st.ListLinks(ctx, org, w.inSprint.ID)
		r("ListLinks", v31, err)
		v32, err := st.GetLink(ctx, org, w.link.ID)
		r("GetLink", v32, err)
		v33, err := st.ListBlockingLinksForDoneTickets(ctx, org, w.sprint.ID)
		r("ListBlockingLinksForDoneTickets", v33, err)
		v34, err := st.ListBlockingLinksFromTicket(ctx, org, w.backlog.ID)
		r("ListBlockingLinksFromTicket", v34, err)
		v35, err := st.BulkGetWaitingOn(ctx, org, w.board.ID)
		r("BulkGetWaitingOn", v35, err)
		v36, err := st.BulkGetBlockedBy(ctx, org, w.board.ID)
		r("BulkGetBlockedBy", v36, err)
		v37, err := st.GetRetroBoard(ctx, org, w.retro.ID)
		r("GetRetroBoard", v37, err)
		v38, err := st.ListRetroBoardsForBoard(ctx, org, w.board.ID)
		r("ListRetroBoardsForBoard", v38, err)
		v39, err := st.BulkCountRetroCards(ctx, org, []uuid.UUID{w.retro.ID})
		r("BulkCountRetroCards", v39, err)
		v40, err := st.ListRetroCards(ctx, org, w.retro.ID)
		r("ListRetroCards", v40, err)
		v41, err := st.GetRetroCard(ctx, org, w.card.ID)
		r("GetRetroCard", v41, err)
		v42, err := st.GetTeam(ctx, org, w.team.ID)
		r("GetTeam", v42, err)
		v43, err := st.ListTeamMembers(ctx, org, w.team.ID)
		r("ListTeamMembers", v43, err)
		v44, err := st.GetAPIToken(ctx, org, w.token.ID)
		r("GetAPIToken", v44, err)
		v45, err := st.ListTicketsByBoard(ctx, org, w.board.ID)
		r("ListTicketsByBoard", v45, err)
		v46, err := st.ListTicketsByColumn(ctx, org, w.col.ID)
		r("ListTicketsByColumn", v46, err)
		v47, err := st.ListTicketsByTeam(ctx, org, w.team.ID)
		r("ListTicketsByTeam", v47, err)
		v48, err := st.GetTicket(ctx, org, w.inSprint.ID)
		r("GetTicket", v48, err)
		v49, err := st.GetTicketAC(ctx, org, w.inSprint.ID)
		r("GetTicketAC", v49, err)
		v50, err := st.ListTicketsByAssignee(ctx, org, w.user.ID)
		r("ListTicketsByAssignee", v50, err)
		v51, err := st.ListActivityByActor(ctx, org, w.user.ID)
		r("ListActivityByActor", v51, err)
		v52, err := st.BulkListTicketAssignees(ctx, org, w.board.ID)
		r("BulkListTicketAssignees", v52, err)
		v53, err := st.ListTicketAssignees(ctx, org, w.inSprint.ID)
		r("ListTicketAssignees", v53, err)
		v54, err := st.GetUserByID(ctx, org, w.user.ID)
		r("GetUserByID", v54, err)
		cm, ck, err := st.GetColumnMeta(ctx, org, w.col.ID)
		r("GetColumnMeta", []string{cm, ck}, err)
	}

	// Sanity: the fixture is real, so "nothing leaked" isn't "nothing loaded".
	own, err := st.ListTicketsByBoard(ctx, org, a.board.ID)
	if err != nil || len(own) != 2 {
		t.Fatalf("org A's own board: %d tickets, err %v; want 2", len(own), err)
	}
	blocked, err := st.BulkGetBlockedBy(ctx, org, a.board.ID)
	if err != nil || len(blocked) != 1 {
		t.Fatalf("org A's own blocked map: %v, err %v; want one entry", blocked, err)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
