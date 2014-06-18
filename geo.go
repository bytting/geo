package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"io/ioutil"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const dateFormat string = "02.01.2006"

type User struct {
	Id       bson.ObjectId `bson:"_id"`
	Username string        `bson:"username"`
	Password string        `bson:"password"`
	Email    string        `bson:"email"`
}

type Sample struct {
	Id          bson.ObjectId `bson:"_id"`
	Activity    float64       `bson:"activity"`
	Uncertainty float64       `bson:"uncertainty"`
	Sigma       int32         `bson:"sigma"`
	RefDate     time.Time     `bson:"refdate"`
	SampleType  string        `bson:"sample_type"`
	Location    struct {
		Coordinates [2]float64 `bson:"coordinates"`
	}
}

type GeoJson struct {
	Type     string
	Features []struct {
		Type       string
		Properties map[string]string
		Geometry   struct {
			Type        string
			Coordinates [][][2]float64
		}
	}
}

func panicIf(e error) {
	if e != nil {
		panic(e)
	}
}

func checkRecover(rw http.ResponseWriter) {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			log.Printf("%v\n", r)
			http.Error(rw, "An error occurred", http.StatusInternalServerError)
		} else {
			log.Println(err.Error())
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
	}
}

func requireAuth(w http.ResponseWriter, r *http.Request, m *mgo.Database) {

	authHeader := r.Header.Get("Authorization")

	if strings.HasPrefix(authHeader, "Basic ") {
		if authItems := strings.Split(authHeader, " "); len(authItems) == 2 && len(authItems[1]) > 0 {
			if authPair, err := base64.StdEncoding.DecodeString(authItems[1]); err == nil {
				if userInfo := strings.Split(string(authPair), ":"); len(userInfo) == 2 {
					u := User{}
					c := m.C("users")
					if err := c.Find(bson.M{"username": userInfo[0], "password": userInfo[1]}).One(&u); err == nil {
						log.Printf("User %s (%s) authenticated successfully\n", u.Username, u.Email)
						return // Return on success
					}
				}
			}
		}
	}

	w.Header().Set("WWW-Authenticate", "Basic realm=\"Authorization Required\"")
	http.Error(w, "Not Authorized", http.StatusUnauthorized)
}

func handleGetRoot(w http.ResponseWriter, rend render.Render) {

	defer checkRecover(w)
	rend.HTML(200, "index", nil)
}

func handleGetSampleTypes(w http.ResponseWriter, r *http.Request, m *mgo.Database) {

	defer checkRecover(w)
	var stypes []string
	c := m.C("data")
	err := c.Find(nil).Distinct("sample_type", &stypes)
	panicIf(err)

	bs, err := json.Marshal(&stypes)
	panicIf(err)

	w.Header().Set("Content-Type", "application/json")
	w.Write(bs)
}

func handleGetSamples(w http.ResponseWriter, r *http.Request, m *mgo.Database) {

	defer checkRecover(w)
	body, err := ioutil.ReadAll(r.Body)
	panicIf(err)

	gj := new(GeoJson)
	err = json.Unmarshal(body, gj)
	panicIf(err)

	c := m.C("data")
	var buf bytes.Buffer
	buf.WriteString("[")

	for n := 0; n < len(gj.Features); n++ {

		query := make(bson.M)
		props := gj.Features[n].Properties
		geom := gj.Features[n].Geometry

		if props["sample_type"] != "" {
			query["sample_type"] = props["sample_type"]
		}
		if props["refdate_from"] != "" && props["refdate_to"] != "" {
			d1, err := time.Parse(dateFormat, props["refdate_from"])
			panicIf(err)
			d2, err := time.Parse(dateFormat, props["refdate_to"])
			panicIf(err)
			query["refdate"] = bson.M{"$gte": d1, "$lt": d2}
		} else {
			if props["refdate_from"] != "" {
				d, err := time.Parse(dateFormat, props["refdate_from"])
				panicIf(err)
				query["refdate"] = bson.M{"$gte": d}
			} else if props["refdate_to"] != "" {
				d, err := time.Parse(dateFormat, props["refdate_to"])
				panicIf(err)
				query["refdate"] = bson.M{"$lt": d}
			}
		}
		query["location.coordinates"] = bson.M{"$within": bson.M{"$polygon": geom.Coordinates[0]}}

		log.Println(geom.Coordinates[0])

		var a []Sample
		err := c.Find(query).Sort("sample_type", "-activity").All(&a)
		panicIf(err)

		for i := 0; i < len(a); i++ {
			b, err := json.Marshal(a[i])
			panicIf(err)
			buf.Write(b)
			buf.WriteString(",")
		}
	}

	if buf.String()[buf.Len()-1] == ',' {
		buf.Truncate(buf.Len() - 1)
	}
	buf.WriteString("]")

	w.Header().Set("Content-Type", "application/json")
	w.Write(buf.Bytes())
}

func main() {

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() { <-c; os.Exit(0) }()

	mongo, err := mgo.Dial("127.0.0.1")
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
	defer mongo.Close()
	//mongo.SetMode(mgo.Monotonic, true)
	db := mongo.DB("geo")

	m := martini.Classic()
	m.Use(render.Renderer())
	m.Map(db)

	m.Get("/api_get_sample_types", requireAuth, handleGetSampleTypes)
	m.Post("/api_get_samples", requireAuth, handleGetSamples)
	m.Get("/", requireAuth, handleGetRoot)

	m.Run()
}
