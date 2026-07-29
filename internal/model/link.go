package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type RelationType string

// Stored relations. The relation_type CHECK constraint accepts these four and
// nothing else. blocks is a hard stop; depends_on only means the other ticket
// comes first, which is why the two are separate rather than one link type.
const (
	RelationBlocks     RelationType = "blocks"
	RelationDependsOn  RelationType = "depends_on"
	RelationDuplicates RelationType = "duplicates"
	RelationRelatesTo  RelationType = "relates_to"
)

// Display-only inverses. A relationship is one row, written from one ticket to
// another; the store swaps it to the inverse when the far ticket reads it.
// Never pass these to CreateLink.
const (
	RelationBlockedBy    RelationType = "blocked_by"
	RelationRequiredBy   RelationType = "required_by"
	RelationDuplicatedBy RelationType = "duplicated_by"
)

// inverseSuffix marks a picker option phrased from the target's side, e.g.
// "blocks_inverse" for "Blocked by". See ParseRelationInput.
const inverseSuffix = "_inverse"

// relationLabels holds every phrase the UI shows for a relation, both directions
// of each pair. Nothing else may spell these out.
var relationLabels = map[RelationType]string{
	RelationBlocks:       "Blocks",
	RelationBlockedBy:    "Blocked by",
	RelationDependsOn:    "Depends on",
	RelationRequiredBy:   "Required by",
	RelationRelatesTo:    "Relates to",
	RelationDuplicates:   "Duplicates",
	RelationDuplicatedBy: "Duplicated by",
}

// inverses pairs each asymmetric relation with how it reads from the far side.
// relates_to is absent because it reads the same both ways.
var inverses = map[RelationType]RelationType{
	RelationBlocks:     RelationBlockedBy,
	RelationDependsOn:  RelationRequiredBy,
	RelationDuplicates: RelationDuplicatedBy,
}

// Label returns the phrase to show, read from the ticket the link is displayed on.
func (r RelationType) Label() string {
	if label, ok := relationLabels[r]; ok {
		return label
	}
	return string(r)
}

// Inverse returns how the relation reads from the far ticket's side.
// Symmetric and unknown relations return themselves.
func (r RelationType) Inverse() RelationType {
	if inv, ok := inverses[r]; ok {
		return inv
	}
	return r
}

// IsStorable reports whether the database accepts the relation.
func (r RelationType) IsStorable() bool {
	_, ok := inverses[r]
	return ok || r == RelationRelatesTo
}

// IsBlocking reports whether the relation is the blocking pair, either
// direction. Blocking links are styled red and cleared when the blocker closes.
func (r RelationType) IsBlocking() bool {
	return r == RelationBlocks || r == RelationBlockedBy
}

// IsDependency reports whether the relation is the sequencing pair, either
// direction. Dependencies are styled amber and outlive the ticket they point at:
// the marker stops showing once it closes, but the link stays.
func (r RelationType) IsDependency() bool {
	return r == RelationDependsOn || r == RelationRequiredBy
}

// ParseRelationInput turns a picker value into the relation to store and whether
// the two tickets must be swapped first. An inverse choice is stored as its
// forward relation with the tickets reversed, keeping one row per relationship.
// ok is false for anything the database would reject.
func ParseRelationInput(raw string) (relation RelationType, swap bool, ok bool) {
	swap = strings.HasSuffix(raw, inverseSuffix)
	relation = RelationType(strings.TrimSuffix(raw, inverseSuffix))
	return relation, swap, relation.IsStorable()
}

// RelationOption is one entry in the link picker.
type RelationOption struct {
	Value string // a stored relation, optionally with the inverse suffix
	Label string
}

// RelationOptions lists the phrasings the link picker offers, in display order:
// both directions of each pair, then relates_to.
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
