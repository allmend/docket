package store

import "strings"

// likeEscaper escapes the LIKE metacharacters so user input matches literally.
// The backslash goes first so the escapes added for % and _ aren't doubled.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// containsPattern turns a search string into a "contains" pattern for
// ILIKE … ESCAPE '\'. Without escaping, "_" matches any character and "%"
// matches everything. Every LIKE built from user input must use this and
// carry the ESCAPE '\' clause.
func containsPattern(q string) string {
	return "%" + likeEscaper.Replace(q) + "%"
}
