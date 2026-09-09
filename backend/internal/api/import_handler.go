package api

import "net/http"

func (s *Server) handleImportProducts(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	// "mode" is an optional multipart form field alongside "file": "overwrite"
	// replaces an existing product's stock outright, anything else (missing
	// field included) adds to it — add is the safer default, since it can't
	// silently erase stock someone else added between exports.
	addToStock := r.FormValue("mode") != "overwrite"

	result, err := s.imports.ImportCSV(r.Context(), file, addToStock)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}
