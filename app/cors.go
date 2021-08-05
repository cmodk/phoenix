package app

import (
	"net/http"

	"github.com/urfave/negroni"
)

func Cors() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Accept-Language, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}
