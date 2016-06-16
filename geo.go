// Interactive map for sample data
// Copyright (C) 2016  Dag Robole
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, write to the Free Software Foundation, Inc.,
// 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
//
// Authors: Dag Robole,

package main

import (
	"encoding/json"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
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

var db *mgo.Database

func panicIf(e error) {
	if e != nil {
		panic(e)
	}
}

func handleGetRoot(c *gin.Context) {

	c.HTML(200, "index.tmpl", nil)
}

func handleGetSampleTypes(c *gin.Context) {

	//defer checkRecover(w)
	var stypes []string
	coll := db.C("data")
	err := coll.Find(nil).Distinct("sample_type", &stypes)
	panicIf(err)

	c.JSON(200, stypes)
}

func handleGetSamples(c *gin.Context) {

	//defer checkRecover(w)
	body, err := ioutil.ReadAll(c.Request.Body)
	panicIf(err)

	gj := new(GeoJson)
	err = json.Unmarshal(body, gj)
	panicIf(err)

	coll := db.C("data")
	var samples []Sample

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

		//log.Println(geom.Coordinates[0])

		var a []Sample
		err := coll.Find(query).Sort("sample_type", "-activity").All(&a)
		panicIf(err)

		samples = append(samples, a...)
	}

	c.JSON(200, samples)
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
	db = mongo.DB("geo")

	r := gin.Default()
	//r.Use(static.Serve("/public"))
	r.Use(static.Serve("/", static.LocalFile("public", false)))
	r.LoadHTMLGlob("templates/*.tmpl")

	r.GET("/api_get_sample_types", handleGetSampleTypes)
	r.POST("/api_get_samples", handleGetSamples)
	r.GET("/", handleGetRoot)

	r.Run(":3000")
}
