package profanity

// denyList is a minimal list of the worst-case slurs and profanity across
// English, Spanish, and Portuguese. It is intentionally kept small — the goal
// is to block obvious hostile nicknames, not to build a comprehensive filter.
// All entries are already normalised (lowercase, no leet substitutions).
var denyList = []string{
	// English
	"nigger", "nigga", "faggot", "fag", "chink", "spic", "kike", "tranny",
	"retard", "cunt", "whore", "slut",
	// Spanish / Portuguese
	"maricón", "maricon", "puta", "coño", "concha", "polla", "pendejo",
	"chingada", "cabrón", "cabron", "mierda", "negro", "negra", "travelo",
	"viado", "buceta", "piroca", "porra", "caralho", "merda", "fodase",
}
