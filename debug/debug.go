package debug

import (
	"net/http/pprof"

	"github.com/gorilla/mux"
)

func Router() *mux.Router {
	prof := mux.NewRouter()
	prof.HandleFunc("/debug/pprof/", pprof.Index).Methods("GET")
	prof.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline).Methods("GET")
	prof.HandleFunc("/debug/pprof/profile", pprof.Profile).Methods("GET")
	prof.HandleFunc("/debug/pprof/symbol", pprof.Symbol).Methods("GET")
	prof.HandleFunc("/debug/pprof/trace", pprof.Trace).Methods("GET")
	return prof
}
