package handlers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"gopkg.in/mgo.v2"
)

func ErrorWithJSON(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, "{\"message\": %q}", message)
}

func ResponseWithJSON(w http.ResponseWriter, json []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(json)
}

func Router(session *mgo.Session) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/api/tickets", allTickets(session)).Methods("GET")
	r.HandleFunc("/api/ticket/{number}", ticketByNumber(session)).Methods("GET")
	r.HandleFunc("/api/tickets/active", activeTickets(session)).Methods("GET")
	r.HandleFunc("/api/tickets/queued", queuedTickets(session)).Methods("GET")
	r.HandleFunc("/api/tickets/report/{year}/{week}", reportTickets(session)).Methods("GET")
	r.HandleFunc("/api/tickets/reportclosed/{year}/{week}", reportClosedTickets(session)).Methods("GET")
	r.HandleFunc("/api/backlog/{date}", reportBacklogTickets(session)).Methods("GET")
	r.HandleFunc("/api/ticket", addTicket(session)).Methods("POST")
	r.HandleFunc("/api/ticket/{number}", updateTicket(session)).Methods("PUT")
	r.HandleFunc("/api/tickets/{number}", deleteTicket(session)).Methods("DELETE")

	//users
	r.HandleFunc("/api/workload", workload(session)).Methods("GET")
	r.HandleFunc("/api/users", allUsers(session)).Methods("GET")
	r.HandleFunc("/api/users/active", activeUsers(session)).Methods("GET")
	r.HandleFunc("/api/users/blacklisted", blacklistedUsers(session)).Methods("GET")
	r.HandleFunc("/api/users/admins", adminsUsers(session)).Methods("GET")
	r.HandleFunc("/api/users/current", currentUser(session)).Methods("GET")
	r.HandleFunc("/api/users/next", nextUser(session)).Methods("GET")
	r.HandleFunc("/api/user/:uid", getUser(session)).Methods("GET")
	r.HandleFunc("/api/attuser/:attuid", getAttUser(session)).Methods("GET")
	r.HandleFunc("/api/user/blacklist/:uid", blacklistUser(session)).Methods("GET")
	r.HandleFunc("/api/user/whitelist/:uid", whitelistUser(session)).Methods("GET")
	r.HandleFunc("/api/user/isadmin/:uid", isAdmin(session)).Methods("GET")
	r.HandleFunc("/api/user", addUser(session)).Methods("POST")
	r.HandleFunc("/api/user/:uid", updateUser(session)).Methods("PUT")
	r.HandleFunc("/api/user/:uid", deleteUser(session)).Methods("DELETE")

	//defects
	r.HandleFunc("/api/defects", searchDefects(session)).Methods("GET")
	//r.HandleFunc("/api/zones", searchZones(session)).Methods("GET")

	//go http.ListenAndServe("0.0.0.0:8083", prof)
	//go http.ListenAndServe("0.0.0.0:8080", mux)
	return r
}
