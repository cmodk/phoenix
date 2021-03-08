package phoenix

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func GetUintParameter(r *http.Request, id string) uint64 {

	parameter := mux.Vars(r)[id]

	value, err := strconv.ParseUint(parameter, 10, 64)
	if err != nil {
		panic(err)
	}

	return value
}
