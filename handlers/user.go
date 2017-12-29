package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os/user"
	"strconv"

	"github.com/gorilla/mux"
	u "github.com/microservices/api/users"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

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

func workload(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

func allUsers(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func activeUsers(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func blacklistedUsers(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func adminsUsers(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func getUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")
		vars := mux.Vars(r)
		uid := vars["uid"]
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
func currentUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func updateUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		uid := vars["uid"]

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
func deleteUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		uid := vars["uid"]

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
func nextUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func whitelistUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		uid := vars["uid"]

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
func blacklistUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		uid := vars["uid"]

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
func isAdmin(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()
		vars := mux.Vars(r)
		uid := vars["uid"]

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
func addUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
func getAttUser(s *mgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("users").C("users")
		vars := mux.Vars(r)
		attuid := vars["attuid"]
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
