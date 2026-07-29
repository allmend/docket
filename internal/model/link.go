package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type RelationType string

// Stored relations — these are the only values the relation_type column accepts.
//
// blocks and depends_on are deliberately separate. A block is a hard stop: the
// blocked ticket cannot proceed. A dependency is sequencing: this needs to happen
// first, but nobody is necessarily stuck. They differ in styling and in what
// happens when the other ticket closes — see IsBlocking / IsDependency.
const (
	RelationBlocks     RelationType = "blocks"
	RelationDependsOn  RelationType = "depends_on"
	RelationDuplicates RelationType = "duplicates"
	RelationRelatesTo  RelationType = "relates_to"
)

// Virtual inverse relations — never stored. A link is written once, from one
// ticket to another; when the *other* ticket displays it, the store rewrites the
// relation to its inverse so the row still reads "this ticket → that ticket".
const (
	RelationBlockedBy    RelationType = "blocked_by"
	RelationRequiredBy   RelationType = "required_by"
	RelationDuplicatedBy RelationType = "duplicated_by"
)

// inverseSuffix marks a picker option that expresses a relation from the target's
// side (e.g. "Blocked by"). Such a choice is stored as the forward relation with
// the two tickets swapped — there is one row per relationship, never two.
const inverseSuffix = "_inverse"

// relationLabels is the single source of truth for relationship phrasing.
// Every asymmetric relation appears here with both of its directions.
var relationLabels = map[RelationType]string{
	RelationBlocks:       "Blocks",
	RelationBlockedBy:    "Blocked by",
	RelationDependsOn:    "Depends on",
	RelationRequiredBy:   "Required by",
	RelationRelatesTo:    "Relates to",
	RelationDuplicates:   "Duplicates",
	RelationDuplicatedBy: "Duplicated by",
}

// inverses pairs each asymmetric stored relation with its display-only inverse.
// relates_to is symmetric and deliberately absent — it reads the same both ways.
var inverses = map[RelationType]RelationType{
	RelationBlocks:     RelationBlockedBy,
	RelationDependsOn:  RelationRequiredBy,
	RelationDuplicates: RelationDuplicatedBy,
}

// Label returns the human-readable phrase for the relation, read from the
// perspective of the ticket the link is displayed on.
func (r RelationType) Label() string {
	if label, ok := relationLabels[r]; ok {
		return label
	}
	return string(r)
}

// Inverse returns how the relation reads from the other ticket's side.
// Symmetric and unknown relations return themselves.
func (r RelationType) Inverse() RelationType {
	if inv, ok := inverses[r]; ok {
		return inv
	}
	return r
}

// IsStorable reports whether the relation is one of the values the database
// accepts. Virtual inverses are not storable.
func (r RelationType) IsStorable() bool {
	_, ok := inverses[r]
	return ok || r == RelationRelatesTo
}

// IsBlocking reports whether the relation is the blocking pair, in either
// direction. Blocking rows are the hard stop: they drive the red blocked badge,
// the dashboard's blocked panel and the blocked metric, and are cleared
// automatically when the blocker closes.
func (r RelationType) IsBlocking() bool {
	return r == RelationBlocks || r == RelationBlockedBy
}

// IsDependency reports whether the relation is the sequencing pair, in either
// direction. Dependencies are softer than blocks: they mark a ticket amber
// rather than red, stay out of the blocked metric, and are never auto-cleared —
// the badge simply stops showing once the depended-on ticket closes.
func (r RelationType) IsDependency() bool {
	return r == RelationDependsOn || r == RelationRequiredBy
}

// ParseRelationInput resolves a value submitted by the link picker into the
// relation to store and whether the two tickets must be swapped first.
// An inverse choice ("blocked_by_inverse") is stored as its forward relation
// with the tickets reversed, so each relationship is exactly one row.
func ParseRelationInput(raw string) (relation RelationType, swap bool, ok bool) {
	swap = strings.HasSuffix(raw, inverseSuffix)
	relation = RelationType(strings.TrimSuffix(raw, inverseSuffix))
	return relation, swap, relation.IsStorable()
}

// RelationOption is one entry in the link picker.
type RelationOption struct {
	Value string // form value: a stored relation, or one with the inverse suffix
	Label string
}

// RelationOptions lists every phrasing a user can pick when adding a link, in
// display order: both directions of each asymmetric pair, then the symmetric one.
func RelationOptions() []RelationOption {
	ordered := []RelationType{
		RelationBlocks, RelationDependsOn, RelationDuplicates,
	}
	opts := make([]RelationOption, 0, len(ordered)*2+1)
	for _, rel := range ordered {
		opts = append(opts,
			RelationOption{Value: string(rel), Label: rel.Label()},
			RelationOption{Value: string(rel) + inverseSuffix, Label: rel.Inverse().Label()},
		)
	}
	return append(opts, RelationOption{
		Value: string(RelationRelatesTo),
		Label: RelationRelatesTo.Label(),
	})
}

type TicketLink struct {
	ID           uuid.UUID    `json:"id"`
	OrgID        uuid.UUID    `json:"org_id"`
	FromTicketID uuid.UUID    `json:"from_ticket_id"`
	ToTicketID   uuid.UUID    `json:"to_ticket_id"`
	Relation     RelationType `json:"relation_type"`
	CreatedAt    time.Time    `json:"created_at"`

	// Denormalised for display — populated by store joins.
	FromDisplayID  string     `json:"from_display_id"`
	FromTitle      string     `json:"from_title"`
	FromClosedAt   *time.Time `json:"from_closed_at,omitempty"`
	FromColumnName string     `json:"from_column_name"`
	ToDisplayID    string     `json:"to_display_id"`
	ToTitle        string     `json:"to_title"`
	ToClosedAt     *time.Time `json:"to_closed_at,omitempty"`
	ToColumnName   string     `json:"to_column_name"`
}
