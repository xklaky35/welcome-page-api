package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
	_ "time/tzdata"

	"github.com/xklaky35/welcome-page-api/db"

	"github.com/gin-gonic/gin"
)


const TIME_FORMAT string = time.RFC3339
const LOG_PATH string = "./local/logs.log"
const DATA_PATH string = ""
const CONFIG_PATH string = "./local/config.json"

// the request needs to bind to a datastructure
// easier this way
type gauge struct {
	Name string `form:"name" json:"name"`
}
func (gauge *gauge) Validate() bool {
	regex := `[^a-zA-Z]+`
	re := regexp.MustCompile(regex)
	if re.MatchString(gauge.Name){
		return false
	}
	return true
}


type Config struct {
	MaxValue int `json:"max_value"`
	MinValue int `json:"min_value"`
	IncreaseStep int `json:"increase_step"`
	DecreaseStep int `json:"decrease_step"`
	Timezone string `json:"timezone"`
}
var conf Config



func main() {
	r := gin.Default()
	
	if !Init() {
		fmt.Println("Init error")
		return
	}

	r.GET("/GetData", getData)
	r.POST("/UpdateGauge", update) //param
	r.POST("/AddGauge", addGauge) //body
	r.POST("/RemoveGauge", removeGauge) //body
	r.POST("/DailyCycle", dailyCycle)

	db.Init()

	r.Run(":3001")
}

func Init() bool {

	var err error
	conf, err = loadConfig()
	if err != nil {
		log.Fatalf("error opening file: %v", err)
		return false
	}

	f, err := os.OpenFile(LOG_PATH, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
		return false
	}
	log.SetOutput(f)
	return true
}

func getData(c *gin.Context){
	data, err := db.LoadData()
	if err != nil {
		fmt.Println(err)
		return
	}
	c.JSON(200, &data)
}

func update(c *gin.Context){
	loc, err := time.LoadLocation(conf.Timezone)
	if err != nil {
		fmt.Println(err)
		return
	}

	// request validation
	var reqGauge gauge
	if err := c.ShouldBindQuery(&reqGauge); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if !reqGauge.Validate(){
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	gauge := db.GetGauge(reqGauge.Name)
	if (db.Gauge{}) == gauge {
		c.AbortWithStatus(http.StatusNotFound)	
		return
	}
	//if isToday(gauge.LastIncrease) {
		//c.AbortWithStatus(http.StatusForbidden)
		//return
	//}

	err = db.UpdateGauge(reqGauge.Name, time.Now().In(loc).Format(TIME_FORMAT), conf.IncreaseStep, conf.MinValue, conf.MaxValue)
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
}

func addGauge(c *gin.Context){

	//-------------------------
	// check abort requirements
	// request valid?
	var reqGauge gauge
	if err := c.ShouldBind(&reqGauge); err != nil {
		fmt.Println(err)
		c.AbortWithStatus(http.StatusBadRequest)
		return 
	}
	// name valid?
	if !reqGauge.Validate(){
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	// already exists?
	if db.GetGauge(reqGauge.Name) != (db.Gauge{}) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// execute logic
	err := db.AddGauge(db.Gauge{
		Name: reqGauge.Name,
		Value: 0,
		LastIncrease: "",
	})
	if err != nil{
		fmt.Println(err)
	}
}

func removeGauge(c *gin.Context){
	var reqGauge gauge
	if err := c.ShouldBind(&reqGauge); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if !reqGauge.Validate(){
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if db.GetGauge(reqGauge.Name) == (db.Gauge{}) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	err := db.RemoveGauge(reqGauge.Name)
	if err != nil {
		return 
	}
}


func dailyCycle(c *gin.Context){
	data, err := db.LoadData()
	if err != nil {
		log.Print()
	}

	for _, e := range data.Gauges {
//		if isToday(e.LastIncrease) {
			//continue
		//}
		err := db.UpdateGauge(e.Name, e.LastIncrease, -conf.DecreaseStep, conf.MinValue, conf.MaxValue)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func isToday(date string) bool{
	t, err := time.Parse(TIME_FORMAT,date)		
	if err != nil {
		log.Print(err)	
		return false
	}
	loc, err := time.LoadLocation(conf.Timezone)
	if err != nil {
		log.Print(err)
		return false
	}

	if t.Day() != time.Now().In(loc).Day(){
		return false
	}
	return true
}

func loadConfig() (Config, error){
	var config Config
	f, err := os.ReadFile(CONFIG_PATH)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(f, &config)

	return config, nil
}
