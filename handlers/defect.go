package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	defect "github.com/microservices/api/defects"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func addDefect(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func getDefect(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("info").C("defect")
		vars := mux.Vars(r)
		id := vars["defect"]
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
func searchDefects(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		c := session.DB("info").C("tickets")
		re := regexp.MustCompile(`https://sdp.web.att.com\S{50,83}(/|=)[\d]{6}`)
		defectNum := regexp.MustCompile(`[\d]{6}`)
		var defects []defect.Defect
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
