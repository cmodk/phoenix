package app

import (
	"encoding/json"
	"net/http"
)

func (app *App) JsonResponse(w http.ResponseWriter, data interface{}) error {
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		app.HttpInternalError(w, err)
		return err
	}

	return nil
}
