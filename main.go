package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

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

type ITH struct {
	State      string    `json:"state"`
	Time       string    `json:"time"`
	ISODate    time.Time `json:"isodate"`
	ModifiedBy string    `json:"modifiedby"`
	Activity   string    `json:"activity"`
}
type TicketLog struct {
	Date    string    `json:"date"`
	ISODate time.Time `json:"isodate"`
	Info    string    `json:"info"`
	User    string    `json:"user"`
}
type TicketParent struct {
	Status     string    `json:"status"`
	Sev        string    `json:"sev"`
	Number     string    `json:"number"`
	Created    string    `json:"created"`
	ISOCreated time.Time `json:"isocreted"`
}

type Ticket struct {
	Sev             string       `json:"sev"`
	SubrootCause    string       `json:"subroot_cause"`
	Opened          string       `json:"opened"`
	ISOOpened       time.Time    `json:"isoopened"`
	Parent          TicketParent `json:"parent"`
	Handover        string       `json:"handover"`
	Abstract        string       `json:"abstract"`
	Number          string       `json:"number"`
	LastModifiedBy  string       `json:"lastmodifiedby"`
	State           string       `json:"state"`
	LastModified    string       `json:"lastmodified"`
	ISOLastModified time.Time    `json:"isolastmodified"`
	Role            string       `json:"role"`
	Dispatch        string       `json:"dispatch"`
	ISOClosed       time.Time    `json:isoclosed`
	Closed          string       `json:"closed"`
	Owner           string       `json:"owner"`
	RootCause       string       `json:"rootcause"`
	Restored        string       `json:"restored"`
	Ith             []ITH        `json:"ith"`
	Logs            []TicketLog  `json:"logs"`
}
type User struct {
	Name      string `json:"name"`
	Is_Active bool   `json:"is_active"`
	Real_Name string `json:"real_name"`
	Current   bool   `json:"current"`
	Is_Admin  bool   `json:"is_admin"`
	ID        string `json:"id"`
	Engineer  bool   `json:"engineer"`
	Attuid    string `json:"attuid"`
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

type Defect struct {
	Id   string   `bson:"_id"`
	Info []string `json:"info"`
}
type DefectOutput struct {
	Number  string `json:"number"`
	Defects string `json:"defect"`
}

func strToTime(ticket *Ticket) {
	var err error
	if ticket.ISOLastModified, err = time.Parse("2006-01-02 15:04:05", ticket.LastModified); err != nil {
		log.Println("Last Modified: ", err)
	}
	if ticket.ISOClosed, err = time.Parse("01/02/2006 15:04", ticket.Closed); err != nil {
		log.Println("Closed: ", err)
	}
	if ticket.ISOOpened, err = time.Parse("2006-01-02 15:04:05", ticket.Opened); err != nil {
		log.Println("Opened: ", err)
	}
	if ticket.Parent.ISOCreated, err = time.Parse("2006-01-02 15:04:05", ticket.Parent.Created); err != nil {
		log.Println("Parent created: ", err)
	}
	for i, _ := range ticket.Ith {
		if ticket.Ith[i].ISODate, err = time.Parse("2006-01-02 15:04:05", ticket.Ith[i].Time); err != nil {
			log.Println("ITH: ", err)
		}
	}
	for i, _ := range ticket.Logs {
		if ticket.Logs[i].ISODate, err = time.Parse("2006-01-02 15:04:05", ticket.Logs[i].Date); err != nil {
			log.Println("Logs: ", err)
		}
	}
}
func main() {
	host := os.Getenv("MONGO_HOST")
	port := os.Getenv("MONGO_PORT")
	mongo := host + ":" + port
	session, err := mgo.Dial(mongo)
	if err != nil {
		panic(fmt.Sprintf("mongodb connection error %s", err))
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
	// debug
	prof := http.NewServeMux()
	prof.HandleFunc("/debug/pprof/", pprof.Index)
	prof.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	prof.HandleFunc("/debug/pprof/profile", pprof.Profile)
	prof.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	prof.HandleFunc("/debug/pprof/trace", pprof.Trace)
	go http.ListenAndServe("0.0.0.0:8083", prof)
	http.ListenAndServe("0.0.0.0:8080", mux)
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

		var tickets []Ticket
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

		var tickets []Ticket
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

		var tickets []Ticket
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

		var tickets []Ticket
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
		var defects []Defect

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
		var resp []DefectOutput
		// goroutine to increase spead
		for _, defect := range defects {
			var def DefectOutput
			for _, l := range defect.Info {
				s := strings.Replace(l, "\n", "", -1)
				d := re.FindAllStringSubmatch(s, -1)
				def.Number = defect.Id
				for _, link := range d {
					def.Number = defect.Id
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
func removeDuplicatesUnordered(elements []DefectOutput) []DefectOutput {
	encountered := map[DefectOutput]bool{}

	// Create a map of all unique elements.
	for v := range elements {
		encountered[elements[v]] = true
	}

	// Place all keys from the map into a slice.
	result := []DefectOutput{}
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
		var tickets []Ticket
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
		var tickets []Ticket
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

		var ticket Ticket
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

		var ticket Ticket
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
			ticket Ticket
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

		var users []User
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

		var users []User
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

		var users []User
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

		var users []User
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
		var user User
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

		var user User
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
			user User
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
		var user User
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
		var users []User
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
			user User
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
			user User
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
			user User
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
		var user User
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
		var user User
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
		var defect DefectOutput
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&defect)
		if err != nil {
			ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
			return
		}
		c := session.DB("info").C("defect")
		err = c.Insert(defect)
		if err != nil {
			if mgo.IsDup(err) {
				ErrorWithJSON(w, "Defect is already exists", http.StatusBadRequest)
				return
			}

			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", r.URL.Path+"/"+defect.Defects)
		w.WriteHeader(http.StatusCreated)
	}
}
func getDefect(s *mgo.Session) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("info").C("defect")
		id := pat.Param(ctx, "defect")
		var defect DefectOutput
		err := c.Find(bson.M{"defects": id}).One(&defect)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			return
		}

		respBody, err := json.MarshalIndent(defect, "", "  ")
		if err != nil {
			log.Println(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
