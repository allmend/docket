package model

import "testing"

// TestRelationLabelsAndInverses pins the relationship vocabulary: four stored
// relations, each reading correctly from both ends.
func TestRelationLabelsAndInverses(t *testing.T) {
	cases := []struct {
		relation RelationType
		forward  string
		inverse  string
	}{
		{RelationBlocks, "Blocks", "Blocked by"},
		{RelationDependsOn, "Depends on", "Required by"},
		{RelationRelatesTo, "Relates to", "Relates to"},
		{RelationDuplicates, "Duplicates", "Duplicated by"},
	}
	for _, c := range cases {
		if got := c.relation.Label(); got != c.forward {
			t.Errorf("%s.Label() = %q, want %q", c.relation, got, c.forward)
		}
		if got := c.relation.Inverse().Label(); got != c.inverse {
			t.Errorf("%s.Inverse().Label() = %q, want %q", c.relation, got, c.inverse)
		}
	}
}

// TestRelationRelatesToIsSymmetric guards the one relation that must read the
// same from both ends — inverting it must not produce a "relates_to_by".
func TestRelationRelatesToIsSymmetric(t *testing.T) {
	if got := RelationRelatesTo.Inverse(); got != RelationRelatesTo {
		t.Fatalf("relates_to inverted to %q, want itself", got)
	}
}

// TestRemovedRelationsStayGone asserts that relations considered and rejected
// for the vocabulary cannot be stored — the DB CHECK constraint rejects them
// anyway, but this fails fast and documents the decision.
func TestRemovedRelationsStayGone(t *testing.T) {
	for _, r := range []RelationType{"clones", "cloned_by", "causes", "caused_by"} {
		if r.IsStorable() {
			t.Errorf("%s is storable; it was dropped from the vocabulary", r)
		}
	}
}

func TestIsStorable(t *testing.T) {
	storable := []RelationType{
		RelationBlocks, RelationDependsOn, RelationDuplicates, RelationRelatesTo,
	}
	for _, r := range storable {
		if !r.IsStorable() {
			t.Errorf("%s should be storable", r)
		}
	}
	// Virtual inverses are display-only and must never reach the database.
	notStorable := []RelationType{
		RelationBlockedBy, RelationRequiredBy, RelationDuplicatedBy,
		"", "clones", "causes", "nonsense",
	}
	for _, r := range notStorable {
		if r.IsStorable() {
			t.Errorf("%s should not be storable", r)
		}
	}
}

func TestIsBlocking(t *testing.T) {
	for _, r := range []RelationType{RelationBlocks, RelationBlockedBy} {
		if !r.IsBlocking() {
			t.Errorf("%s should count as blocking", r)
		}
	}
	for _, r := range []RelationType{RelationRelatesTo, RelationDuplicates, RelationDependsOn, RelationRequiredBy} {
		if r.IsBlocking() {
			t.Errorf("%s should not count as blocking", r)
		}
	}
}

// TestIsDependencyIsDisjointFromBlocking pins the distinction the two pairs
// exist to express: a dependency is amber and informational, a block is red and
// hard. Nothing may be both, or the card styling becomes ambiguous.
func TestIsDependencyIsDisjointFromBlocking(t *testing.T) {
	for _, r := range []RelationType{RelationDependsOn, RelationRequiredBy} {
		if !r.IsDependency() {
			t.Errorf("%s should count as a dependency", r)
		}
	}
	for _, r := range []RelationType{RelationBlocks, RelationBlockedBy, RelationRelatesTo, RelationDuplicates} {
		if r.IsDependency() {
			t.Errorf("%s should not count as a dependency", r)
		}
	}
	for _, r := range RelationOptions() {
		relation, _, _ := ParseRelationInput(r.Value)
		if relation.IsBlocking() && relation.IsDependency() {
			t.Errorf("%s is both blocking and a dependency", relation)
		}
	}
}

func TestParseRelationInput(t *testing.T) {
	cases := []struct {
		raw      string
		relation RelationType
		swap     bool
		ok       bool
	}{
		{"blocks", RelationBlocks, false, true},
		{"blocks_inverse", RelationBlocks, true, true},
		{"relates_to", RelationRelatesTo, false, true},
		{"depends_on", RelationDependsOn, false, true},
		{"depends_on_inverse", RelationDependsOn, true, true},
		{"duplicates_inverse", RelationDuplicates, true, true},
		// Rejected: removed, virtual, and junk values.
		{"clones", "clones", false, false},
		{"blocked_by", RelationBlockedBy, false, false},
		{"", "", false, false},
		{"nonsense", "nonsense", false, false},
	}
	for _, c := range cases {
		relation, swap, ok := ParseRelationInput(c.raw)
		if relation != c.relation || swap != c.swap || ok != c.ok {
			t.Errorf("ParseRelationInput(%q) = (%q, %v, %v), want (%q, %v, %v)",
				c.raw, relation, swap, ok, c.relation, c.swap, c.ok)
		}
	}
}

// TestRelationOptionsRoundTrip asserts every phrasing offered in the picker
// parses back into something storable — a UI option that 400s is the bug this
// catches.
func TestRelationOptionsRoundTrip(t *testing.T) {
	opts := RelationOptions()
	if len(opts) != 7 {
		t.Fatalf("got %d picker options, want 7 (3 pairs + relates_to)", len(opts))
	}
	seen := make(map[string]bool, len(opts))
	for _, o := range opts {
		if seen[o.Value] {
			t.Errorf("duplicate option value %q", o.Value)
		}
		seen[o.Value] = true
		if o.Label == "" || o.Label == o.Value {
			t.Errorf("option %q has no human label (got %q)", o.Value, o.Label)
		}
		relation, _, ok := ParseRelationInput(o.Value)
		if !ok {
			t.Errorf("picker option %q does not parse to a storable relation", o.Value)
		}
		if !relation.IsStorable() {
			t.Errorf("picker option %q parses to unstorable %q", o.Value, relation)
		}
	}
}
