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
const CONFIG_PATH string = "./local/config.json"

// the request needs to bind to a datastructure
// easier this way
type gauge struct {
	Name string `form:"name" binding:"required"`
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


	r.Run(":3001")
}

func Init() bool {

	// setup logging
	f, err := os.OpenFile(LOG_PATH, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err)
		return false
	}
	log.SetOutput(f)


	// setup configuration
	conf, err = loadConfig()
	if err != nil {
		fmt.Printf("Creating default configuration file...\n")

		// load default config
		conf = Config{
			MaxValue: 10,
			MinValue: 0,
			IncreaseStep: 1,
			DecreaseStep: 2,
			Timezone: "Europe/Berlin",
		}
		data, _ := json.Marshal(&conf)
		os.WriteFile(CONFIG_PATH, data, 0666)
	}

	// setup db
	fmt.Println("Loading DB driver...")
	db.LoadDriver()

	fmt.Println("Checking DB file...")
	f, err = os.OpenFile(db.DB_PATH, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer f.Close()

	fmt.Println("Checking DB schema...")
	exists, err := db.CreateSchema()
	if err != nil {
		return false
	}
	if !exists {
		fmt.Println("Creating new schema...")
	}
	return true
}

func getData(c *gin.Context){
	data, err := db.LoadData()
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
	c.JSON(200, &data)
}

func update(c *gin.Context){
	loc, err := time.LoadLocation(conf.Timezone)
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	// request validation
	var reqGauge gauge
	if err := c.ShouldBindQuery(&reqGauge); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
	}
	if !reqGauge.Validate(){
		c.AbortWithStatus(http.StatusBadRequest)
	}

	gauge := db.GetGauge(reqGauge.Name)
	if (db.Gauge{}) == gauge {
		c.AbortWithStatus(http.StatusNotFound)	
	}
	//if isToday(gauge.LastIncrease) {
		//c.AbortWithStatus(http.StatusForbidden)
		//return
	//}

	err = db.UpdateGauge(reqGauge.Name, time.Now().In(loc).Format(TIME_FORMAT), conf.IncreaseStep, conf.MinValue, conf.MaxValue)
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(http.StatusServiceUnavailable)
	}
}

func addGauge(c *gin.Context){

	//-------------------------
	// check abort requirements
	// request valid?
	var reqGauge gauge
	if err := c.ShouldBind(&reqGauge); err != nil {
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
		log.Println(err)
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
		log.Println(err)
		return 
	}
}


func dailyCycle(c *gin.Context){
	data, err := db.LoadData()
	if err != nil {
		log.Println(err)
	}

	for _, e := range data.Gauges {
//		if isToday(e.LastIncrease) {
			//continue
		//}
		err := db.UpdateGauge(e.Name, e.LastIncrease, -conf.DecreaseStep, conf.MinValue, conf.MaxValue)
		if err != nil {
			log.Println(err)
		}
	}
}

func isToday(date string) bool{
	t, err := time.Parse(TIME_FORMAT,date)		
	if err != nil {
		log.Println(err)	
		return false
	}
	loc, err := time.LoadLocation(conf.Timezone)
	if err != nil {
		log.Println(err)
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
