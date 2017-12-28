package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/microservices/api/debug"
	defect "github.com/microservices/api/defects"
	logs "github.com/microservices/api/logs"
	ticket "github.com/microservices/api/tickets"
	u "github.com/microservices/api/users"
	version "github.com/microservices/api/version"
	log "github.com/sirupsen/logrus"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
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

type Field struct {
	Num   string `json:"num"`
	State string `json:"state"`
	Owner string `json:"owner"`
}
type jobs struct {
	//ID      string  `json:"_id"`
	//Owner   string  `json:"owner"`
	Tickets []Field `json:"tickets"`
}

var logger *log.Entry

func main() {
	host := os.Getenv("MONGO_HOST")
	logLevel := os.Getenv("LOG_LEVEL")
	port := os.Getenv("PORT")
	profilePort := os.Getenv("PROFILE_PORT")
	logger = logs.Logger("api", logLevel)
	log.WithFields(log.Fields{
		"service":  "api",
		"event":    "starting",
		"commit":   version.Commit,
		"build":    version.BuildTime,
		"release":  version.Release,
		"logLevel": logLevel,
		"mongodb":  host,
		"port":     port,
		"profile":  profilePort}).Info("Starting the API service...")

	// host will have hostname:port
	logger.Debug(host)

	if host == "" {
		logger.Fatal("MongoDB not set")
	}
	if port == "" {
		logger.Fatal("Port not set")
	}

	session, err := mgo.Dial(host)
	if err != nil {
		logger.Fatal(err)
	}
	defer session.Close()
	session.SetMode(mgo.Monotonic, true)
	ensureIndex(session)

	mux := goji.NewMux()
	// tickets
	mux.HandleFuncC(pat.Get("/api/tickets"), allTickets(session))
	mux.HandleFuncC(pat.Post("/api/ticket"), addTicket(session))
	mux.HandleFuncC(pat.Get("/api/ticket/:number"), ticketByNumber(session))
	mux.HandleFuncC(pat.Put("/api/ticket/:number"), updateTicket(session))
	mux.HandleFuncC(pat.Delete("/api/tickets/:number"), deleteTicket(session))
	mux.HandleFuncC(pat.Get("/api/tickets/active"), activeTickets(session))
	mux.HandleFuncC(pat.Get("/api/tickets/queued"), queuedTickets(session))
	mux.HandleFuncC(pat.Get("/api/tickets/report/:year/:week"), reportTickets(session))
	mux.HandleFuncC(pat.Get("/api/tickets/reportclosed/:year/:week"), reportClosedTickets(session))
	mux.HandleFuncC(pat.Get("/api/backlog/:date"), reportBacklogTickets(session))
	//users
	mux.HandleFuncC(pat.Post("/api/user"), addUser(session))
	mux.HandleFuncC(pat.Get("/api/workload"), workload(session))
	mux.HandleFuncC(pat.Get("/api/users"), allUsers(session))
	mux.HandleFuncC(pat.Get("/api/users/active"), activeUsers(session))
	mux.HandleFuncC(pat.Get("/api/users/blacklisted"), blacklistedUsers(session))
	mux.HandleFuncC(pat.Get("/api/users/admins"), adminsUsers(session))
	mux.HandleFuncC(pat.Get("/api/users/current"), currentUser(session))
	mux.HandleFuncC(pat.Get("/api/users/next"), nextUser(session))
	mux.HandleFuncC(pat.Put("/api/user/:uid"), updateUser(session))
	mux.HandleFuncC(pat.Get("/api/user/:uid"), getUser(session))
	mux.HandleFuncC(pat.Get("/api/attuser/:attuid"), getAttUser(session))
	mux.HandleFuncC(pat.Delete("/api/user/:uid"), deleteUser(session))
	mux.HandleFuncC(pat.Get("/api/user/blacklist/:uid"), blacklistUser(session))
	mux.HandleFuncC(pat.Get("/api/user/whitelist/:uid"), whitelistUser(session))
	mux.HandleFuncC(pat.Get("/api/user/isadmin/:uid"), isAdmin(session))
	//defects
	mux.HandleFuncC(pat.Get("/api/defects"), searchDefects(session))
	mux.HandleFuncC(pat.Get("/api/zones"), searchZones(session))
	if profilePort != "" {
		prof := debug.Router()
		go http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", profilePort), prof)
	}

	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", port), mux)
}

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

func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	c := session.DB("info").C("tickets")

	index := mgo.Index{
		Key:        []string{"number"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err := c.EnsureIndex(index)
	if err != nil {
		log.Println(err)
	}
}

func allTickets(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
func activeTickets(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
func reportBacklogTickets(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		date := pat.Param(ctx, "date")
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
func queuedTickets(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
func workload(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		c := session.DB("info").C("tickets")
		/*
			db.tickets.aggregate(
				[{$project: {
					_id:0,
					owner:"$owner",
					num:"$number",
					status:"$state"}},
				{$match:{
					"status": {$ne:"Closed"}}},
				{$group: {
					_id:"$owner",
					num:{$push:{num:"$num", state:"$status"}},
					total:{ $sum : 1 }}}])
		*/
		pipe := c.Pipe([]bson.M{{"$project": bson.M{"number": "$number", "owner": "$owner", "state": "$state"}}, {"$match": bson.M{"$and": []bson.M{bson.M{"state": bson.M{"$ne": "Cancel"}}, bson.M{"state": bson.M{"$ne": "Closed"}}}}}, {"$group": bson.M{"_id": "$owner", "tickets": bson.M{"$push": bson.M{"num": "$number", "state": "$state", "owner": "$owner"}}}}})
		var workloads []jobs
		err := pipe.All(&workloads)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Printf("%v", err)
			return
		}
		respBody, err := json.MarshalIndent(workloads, "", "  ")
		if err != nil {
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func searchDefects(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		c := session.DB("info").C("tickets")
		re := regexp.MustCompile(`https://sdp.web.att.com\S{50,83}(/|=)[\d]{6}`)
		defectNum := regexp.MustCompile(`[\d]{6}`)
		var defects []defect.Defect

		// add finding only not closed tickets
		//		b.tickets.aggregate(
		//		{ $match: {'state': /.*/}},
		//		{ $unwind: '$logs'},
		//		{ $match: {'logs.info':
		//		{$regex: /.*Workit/,$options: "sim"}}},
		//	{ $group: {_id: '$number', list: {$push: '$logs.info'}}})
		//
		//err := c.Find(bson.M{"logs.info": bson.M{"$regex": bson.RegEx{`.*Workitem.*`, "sim"}}}).Select(bson.M{"number": 1, "owner": 1, "logs": 1}).All(&tickets)
		// {"$match": bson.M{"$and": []bson.M{bson.M{"state": bson.M{"$ne": "Cancel"}}, bson.M{"state": bson.M{"$ne": "Closed"}}}}},
		pipe := c.Pipe([]bson.M{{"$match": bson.M{"$and": []bson.M{bson.M{"state": bson.M{"$ne": "Cancel"}}, bson.M{"state": bson.M{"$ne": "Closed"}}}}},
			{"$unwind": "$logs"},
			{"$match": bson.M{"logs.info": bson.M{"$regex": bson.RegEx{`.*Workitem.*`, "sim"}}}},
			{"$group": bson.M{"_id": "$number", "info": bson.M{"$push": "$logs.info"}}}})
		err := pipe.All(&defects)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Println("Failed get report: ", err)
			return
		}
		var resp []defect.DefectOutput
		// goroutine to increase spead
		for _, d := range defects {
			var def defect.DefectOutput
			for _, l := range d.Info {
				s := strings.Replace(l, "\n", "", -1)
				out := re.FindAllStringSubmatch(s, -1)
				def.Number = d.Id
				for _, link := range out {
					def.Number = d.Id
					def.Defects = defectNum.FindString(string(link[0]))
					resp = append(resp, def)
				}
			}
		}

		resp = removeDuplicatesUnordered(resp)
		respBody, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func searchZones(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		c := session.DB("info").C("tickets")
		//re := regexp.MustCompile(`CLOUD_ZONE_.*`)
		var zones []interface{}
		pipe := c.Pipe([]bson.M{{"$match": bson.M{"$and": []bson.M{bson.M{"state": bson.M{"$ne": "Cancel"}}, bson.M{"state": bson.M{"$ne": "Closed"}}}}},
			{"$unwind": "$logs"},
			{"$match": bson.M{"logs.info": bson.M{"$regex": bson.RegEx{`.*.*`, "sim"}}}},
			{"$group": bson.M{"_id": "$number", "info": bson.M{"$push": "$logs.info"}}}})
		err := pipe.All(&zones)
		respBody, err := json.MarshalIndent(zones, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func removeDuplicatesUnordered(elements []defect.DefectOutput) []defect.DefectOutput {
	encountered := map[defect.DefectOutput]bool{}

	// Create a map of all unique elements.
	for v := range elements {
		encountered[elements[v]] = true
	}

	// Place all keys from the map into a slice.
	result := []defect.DefectOutput{}
	for key, _ := range encountered {
		result = append(result, key)
	}
	return result
}
func reportTickets(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		var (
			year, week int64
			err        error
		)
		if week, err = strconv.ParseInt(pat.Param(ctx, "week"), 10, 32); err != nil {
			log.Println(err)
		}
		if year, err = strconv.ParseInt(pat.Param(ctx, "year"), 10, 32); err != nil {
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
func reportClosedTickets(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		var (
			year, week int64
			err        error
		)
		if week, err = strconv.ParseInt(pat.Param(ctx, "week"), 10, 32); err != nil {
			log.Println(err)
		}
		if year, err = strconv.ParseInt(pat.Param(ctx, "year"), 10, 32); err != nil {
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
func addTicket(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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

func ticketByNumber(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		number := pat.Param(ctx, "number")

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

func updateTicket(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		number := pat.Param(ctx, "number")

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

func deleteTicket(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		number := pat.Param(ctx, "number")

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

func allUsers(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")

		var users []user.User
		err := c.Find(bson.M{"engineer": true}).All(&users)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Println("Failed get all users: ", err)
			return
		}

		respBody, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func activeUsers(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")

		var users []user.User
		err := c.Find(bson.M{"$and": []bson.M{bson.M{"is_active": true}, bson.M{"engineer": true}}}).All(&users)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func blacklistedUsers(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")

		var users []user.User
		err := c.Find(bson.M{"$and": []bson.M{bson.M{"is_active": false}, bson.M{"engineer": true}}}).All(&users)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func adminsUsers(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")

		var users []user.User
		err := c.Find(bson.M{"is_admin": true}).All(&users)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func getUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")
		uid := pat.Param(ctx, "uid")
		var user user.User
		err := c.Find(bson.M{"id": uid}).One(&user)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(user, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func currentUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")

		var user user.User
		err := c.Find(bson.M{"current": true}).One(&user)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(user, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func updateUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		uid := pat.Param(ctx, "uid")

		var (
			user u.User
			err  error
		)
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&user)
		if err != nil {
			ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
			return
		}
		c := session.DB("users").C("users")
		err = c.Update(bson.M{"id": uid}, &user)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "User not found", http.StatusNotFound)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
func deleteUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		uid := pat.Param(ctx, "uid")

		c := session.DB("users").C("users")
		var user u.User
		err := c.Find(bson.M{"id": uid}).One(&user)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "User not found", http.StatusNotFound)
				return
			}
		}
		log.Println(user.Real_Name, " - ", user.Current)
		if user.Current == true {
			log.Println("This user is current. Please execute next user before delete this")
			ErrorWithJSON(w, "This user is current. Please execute next user before delete this", http.StatusInternalServerError)
			return
		}
		err = c.Remove(bson.M{"id": uid})
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				log.Println("Failed delete user: ", err)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "User not found", http.StatusNotFound)
				return
			}
		}
		respBody, err := json.MarshalIndent(user, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func nextUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		c := session.DB("users").C("users")
		var users []u.User
		err := c.Find(bson.M{"$and": []bson.M{bson.M{"is_active": true}, bson.M{"engineer": true}}}).All(&users)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Println("Failed get active users for next: ", err)
			return
		}
		count := len(users)
		for i, user := range users {
			if user.Current == true {
				user.Current = false
				users[(i+1)%count].Current = true
				err = c.Update(bson.M{"id": user.ID}, &user)
				if err != nil {
					switch err {
					default:
						ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
						return
					case mgo.ErrNotFound:
						ErrorWithJSON(w, "User not found", http.StatusNotFound)
						return
					}
				}
				nextUser := users[(i+1)%count]
				err = c.Update(bson.M{"id": nextUser.ID}, &nextUser)
				if err != nil {
					switch err {
					default:
						ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
						return
					case mgo.ErrNotFound:
						ErrorWithJSON(w, "User not found", http.StatusNotFound)
						return
					}
				}
				respBody, err := json.MarshalIndent(nextUser, "", "  ")
				if err != nil {
					log.Println(err)
				}
				ResponseWithJSON(w, respBody, http.StatusOK)
				return
			}
		}
	}
}
func whitelistUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		uid := pat.Param(ctx, "uid")

		var (
			user u.User
			err  error
		)
		c := session.DB("users").C("users")
		err = c.Find(bson.M{"id": uid}).One(&user)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "User not found", http.StatusNotFound)
				return
			}
		}
		user.Is_Active = true
		err = c.Update(bson.M{"id": uid}, &user)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "User not found", http.StatusNotFound)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
func blacklistUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		uid := pat.Param(ctx, "uid")

		var (
			user u.User
			err  error
		)
		c := session.DB("users").C("users")
		err = c.Find(bson.M{"id": uid}).One(&user)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "User not found", http.StatusNotFound)
				return
			}
		}
		log.Println(user.Real_Name, " - ", user.Current)
		if user.Current == true {
			ErrorWithJSON(w, "This user is current. Please execute next user before blacklist this", http.StatusInternalServerError)
			return
		}
		user.Is_Active = false
		err = c.Update(bson.M{"id": uid}, &user)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "User not found", http.StatusNotFound)
				return
			}
		}
	}
}
func isAdmin(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		uid := pat.Param(ctx, "uid")

		var (
			user u.User
			err  error
		)
		c := session.DB("users").C("users")
		err = c.Find(bson.M{"id": uid}).One(&user)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "User not found", http.StatusNotFound)
				return
			}
		}
		ResponseWithJSON(w, []byte(strconv.FormatBool(user.Is_Admin)), http.StatusOK)
	}
}
func addUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		var (
			err error
		)
		var user u.User
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&user)
		if err != nil {
			ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
			return
		}
		c := session.DB("users").C("users")
		err = c.Insert(user)
		if err != nil {
			if mgo.IsDup(err) {
				ErrorWithJSON(w, "User with this uid already exists", http.StatusBadRequest)
				return
			}

			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", r.URL.Path+"/"+user.Real_Name)
		w.WriteHeader(http.StatusCreated)
	}
}
func getAttUser(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")
		attuid := pat.Param(ctx, "attuid")
		var user u.User
		err := c.Find(bson.M{"attuid": attuid}).One(&user)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(user, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
func addDefect(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		var (
			err error
		)
		var d defect.DefectOutput
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&d)
		if err != nil {
			ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
			return
		}
		c := session.DB("info").C("defect")
		err = c.Insert(d)
		if err != nil {
			if mgo.IsDup(err) {
				ErrorWithJSON(w, "Defect is already exists", http.StatusBadRequest)
				return
			}

			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", r.URL.Path+"/"+d.Defects)
		w.WriteHeader(http.StatusCreated)
	}
}
func getDefect(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("info").C("defect")
		id := pat.Param(ctx, "defect")
		var d defect.DefectOutput
		err := c.Find(bson.M{"defects": id}).One(&d)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
