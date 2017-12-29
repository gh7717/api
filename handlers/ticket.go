package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	ticket "github.com/microservices/api/tickets"
	log "github.com/sirupsen/logrus"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var logger *log.Entry

func strToTime(ticket *ticket.Ticket) {
	var err error
	if ticket.ISOLastModified, err = time.Parse("2006-01-02 15:04:05", ticket.LastModified); err != nil {

	}
	if ticket.ISOClosed, err = time.Parse("01/02/2006 15:04", ticket.Closed); err != nil {
		logger.Error(err)
	}
	if ticket.ISOOpened, err = time.Parse("2006-01-02 15:04:05", ticket.Opened); err != nil {
		logger.Error(err)
	}
	if ticket.Parent.ISOCreated, err = time.Parse("2006-01-02 15:04:05", ticket.Parent.Created); err != nil {
		logger.Error(err)
	}
	for i, _ := range ticket.Ith {
		if ticket.Ith[i].ISODate, err = time.Parse("2006-01-02 15:04:05", ticket.Ith[i].Time); err != nil {
			logger.Error(err)
		}
	}
	for i, _ := range ticket.Logs {
		if ticket.Logs[i].ISODate, err = time.Parse("2006-01-02 15:04:05", ticket.Logs[i].Date); err != nil {
			logger.Error(err)
		}
	}
}

func allTickets(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("info").C("tickets")

		var tickets []ticket.Ticket
		err := c.Find(bson.M{}).Sort("isoopened").All(&tickets)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(tickets, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func sel(q ...string) (r bson.M) {
	r = make(bson.M, len(q))
	for _, s := range q {
		r[s] = 1
	}
	return
}
func activeTickets(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("info").C("tickets")

		var tickets []ticket.Ticket
		// db.tickets.find({"state": {$ne: "Closed"}}, {number:1, state:1, owner:1, sev:1})
		err := c.Find(bson.M{"$and": []bson.M{bson.M{"state": bson.M{"$ne": "Cancel"}}, bson.M{"state": bson.M{"$ne": "Closed"}}}}).Select(sel("number", "owner", "sev", "state", "isoopened", "abstract", "isolastmodified")).Sort("isoopened").All(&tickets)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}
		respBody, err := json.MarshalIndent(tickets, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func reportBacklogTickets(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		date := vars["date"]
		ISODate, err := time.Parse("2006-01-02", date)
		if err != nil {
			log.Println("Can't parse date", err)
		}
		c := session.DB("info").C("tickets")

		var tickets []ticket.Ticket
		//db.tickets.find({$and:[{isoclosed:{$gt:ISODate("2017-01-31T00:00:00.000Z")}}},{isoopened:{$lte:ISODate("2017-01-31T00:00:00.000Z")}}],{number:1,isoopened:1,isoclosed:1 })
		err = c.Find(bson.M{"$and": []bson.M{bson.M{"$or": []bson.M{bson.M{"state": bson.M{"$ne": "Closed"}}, bson.M{"isoclosed": bson.M{"$gte": ISODate}}}}, bson.M{"isoopened": bson.M{"$lte": ISODate}}}}).Select(sel("number", "owner", "sev", "state", "isoopened", "abstract", "isoclosed")).Sort("isoopened").All(&tickets)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}
		respBody, err := json.MarshalIndent(tickets, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func queuedTickets(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("info").C("tickets")

		var tickets []ticket.Ticket
		// db.tickets.find({"state": {$ne: "Closed"}}, {number:1, state:1, owner:1, sev:1})
		err := c.Find(bson.M{"state": "Queued"}).Select(sel("number", "owner", "sev", "state", "isolastmodified", "abstract")).All(&tickets)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(tickets, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}

func reportTickets(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		var (
			year, week int64
			err        error
		)
		vars := mux.Vars(r)
		if week, err = strconv.ParseInt(vars["week"], 10, 32); err != nil {
			log.Println(err)
		}
		if year, err = strconv.ParseInt(vars["year"], 10, 32); err != nil {
			log.Println(err)
		}
		c := session.DB("info").C("tickets")
		//project := bson.M{"$project": bson.M{"dayofweek":bson.M{"$dayOfWeek": "$isocreated"}}, "year":bson.M{"$year":"$isocreated"}, "week":bson.M{"$week":"$isocreated"},"number":"$number", "owner":"$owner", "state":"$state", "_id": 0}
		//match := bson.M{"$match": bson.M{"$and": []interface{}{bson.M{"week":week}, bson.M{"year":year}}}, "_id":0}
		//operations := []bson.M{project, match}
		//pipe := c.Pipe(operations)
		pipe := c.Pipe([]bson.M{{"$project": bson.M{"number": "$number", "owner": "$owner", "isoopened": "$isoopened", "state": "$state", "week": bson.M{"$week": "$isoopened"}, "year": bson.M{"$year": "$isoopened"}}}, {"$match": bson.M{"$and": []interface{}{bson.M{"week": week}, bson.M{"year": year}}}}})
		var tickets []ticket.Ticket
		err = pipe.All(&tickets)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Println("Failed get report: ", err)
			return
		}
		respBody, err := json.MarshalIndent(tickets, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func reportClosedTickets(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		var (
			year, week int64
			err        error
		)
		vars := mux.Vars(r)
		if week, err = strconv.ParseInt(vars["week"], 10, 32); err != nil {
			log.Println(err)
		}
		if year, err = strconv.ParseInt(vars["year"], 10, 32); err != nil {
			log.Println(err)
		}
		c := session.DB("info").C("tickets")
		//project := bson.M{"$project": bson.M{"dayofweek":bson.M{"$dayOfWeek": "$isocreated"}}, "year":bson.M{"$year":"$isocreated"}, "week":bson.M{"$week":"$isocreated"},"number":"$number", "owner":"$owner", "state":"$state", "_id": 0}
		//match := bson.M{"$match": bson.M{"$and": []interface{}{bson.M{"week":week}, bson.M{"year":year}}}, "_id":0}
		//operations := []bson.M{project, match}
		//pipe := c.Pipe(operations)
		pipe := c.Pipe([]bson.M{{"$project": bson.M{"number": "$number", "owner": "$owner", "isoclosed": "$isoclosed", "week": bson.M{"$week": "$isoclosed"}, "year": bson.M{"$year": "$isoclosed"}}}, {"$match": bson.M{"$and": []interface{}{bson.M{"week": week}, bson.M{"year": year}}}}})
		var tickets []ticket.Ticket
		err = pipe.All(&tickets)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}
		respBody, err := json.MarshalIndent(tickets, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func addTicket(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		var ticket ticket.Ticket
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&ticket)
		strToTime(&ticket)
		if err != nil {
			ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
			return
		}

		c := session.DB("info").C("tickets")

		err = c.Insert(ticket)
		if err != nil {
			if mgo.IsDup(err) {
				ErrorWithJSON(w, "Ticket with this number already exists", http.StatusBadRequest)
				return
			}

			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", r.URL.Path+"/"+ticket.Number)
		w.WriteHeader(http.StatusCreated)
	}
}

func ticketByNumber(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		number := vars["number"]

		c := session.DB("info").C("tickets")

		var ticket ticket.Ticket
		err := c.Find(bson.M{"number": number}).One(&ticket)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		if ticket.Number == "" {
			ErrorWithJSON(w, "Ticket not found", http.StatusNotFound)
			return
		}

		respBody, err := json.MarshalIndent(ticket, "", "  ")
		if err != nil {
			log.Println(err)
		}

		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}

func updateTicket(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		number := vars["number"]

		var (
			ticket ticket.Ticket
			err    error
		)
		//requestDump, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Println(err)
		}
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&ticket)
		if err != nil {
			ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
			return
		}
		strToTime(&ticket)
		c := session.DB("info").C("tickets")
		err = c.Update(bson.M{"number": number}, &ticket)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				log.Println("Failed update ticket: ", err)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "Ticket not found", http.StatusNotFound)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteTicket(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		number := vars["number"]

		c := session.DB("info").C("tickets")

		err := c.Remove(bson.M{"number": number})
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				log.Println("Failed delete ticket: ", err)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "Ticket not found", http.StatusNotFound)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
