package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/microservices/api/debug"
	"github.com/microservices/api/handlers"
	logs "github.com/microservices/api/logs"
	version "github.com/microservices/api/version"
	log "github.com/sirupsen/logrus"
	"goji.io"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

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

	r := handlers.Router(session)
	if profilePort != "" {
		prof := debug.Router()
		go http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", profilePort), prof)
	}

	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", port), r)
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
