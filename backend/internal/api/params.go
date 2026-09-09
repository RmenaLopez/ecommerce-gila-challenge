package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

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

// parseIntParam reads an optional integer query parameter. A missing or
// empty value returns 0 (letting the service layer apply its own default);
// a present but non-numeric value writes a 400 and returns ok=false.
func parseIntParam(w http.ResponseWriter, query url.Values, name string) (n int, ok bool) {
	raw := query.Get(name)
	if raw == "" {
		return 0, true
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s", name))
		return 0, false
	}
	return n, true
}
