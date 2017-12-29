package zones

import (
	"encoding/json"
	"log"
	"net/http"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func searchZones(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
