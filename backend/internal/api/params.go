package api

import (
	"net/http"

	"github.com/google/uuid"
)

// parseIDParam validates the "id" path value is a well-formed UUID before
// any service/repository call is made, avoiding a wasted database round
// trip on obviously malformed input. Writes a 400 response and returns
// ok=false if invalid.
func parseIDParam(w http.ResponseWriter, r *http.Request) (id string, ok bool) {
	id = r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return "", false
	}
	return id, true
}
