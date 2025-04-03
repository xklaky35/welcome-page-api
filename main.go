package main 

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
	_"time/tzdata"
	"encoding/json"

	"github.com/xklaky35/welcome-page-api/db"

	"github.com/gin-gonic/gin"
)


const TIME_FORMAT string = time.RFC3339
const LOG_PATH string = "./local/logs.log"
const DATA_PATH string = ""
const CONFIG_PATH string = "./local/config.json"

type gauge struct {
	Name string `form:"name" json:"name"`
}

type Config struct {
	MaxValue int `json:"max_value"`
	MinValue int `json:"min_value"`
	IncreaseStep int `json:"increase_step"`
	DecreaseStep int `json:"decrease_step"`
	Timezone string `json:"timezone"`
}
var conf Config

func (gauge *gauge) Validate() bool {
	regex := `[^a-zA-Z]+`
	re := regexp.MustCompile(regex)
	if re.MatchString(gauge.Name){
		return false
	}
	return true
}


func main() {
	r := gin.Default()
	
//	if !Init() {
//		return
//	}

//	r.GET("/GetData", getData)
//	r.POST("/UpdateGauge", update) //param
	r.POST("/AddGauge", addGauge) //body
//	r.POST("/RemoveGauge", removeGauge) //body
//	r.POST("/DailyCycle", dailyCycle)

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
	data, err := db.LoadData(DATA_PATH)
	if err != nil {
		fmt.Println(err)
		return
	}

	c.JSON(200, &data)
}

func update(c *gin.Context){
	data, err := db.LoadData(DATA_PATH)
	if err != nil {
		log.Println(err)
		return
	}

	var reqGauge gauge
	if err := c.ShouldBindQuery(&reqGauge); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !reqGauge.Validate(){
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// search for the gauge and increase it if found
	if i, exists := findGauge(data.Gauges, reqGauge.Name); exists == true{
		err := increase(&data.Gauges[i])
		if err != nil {
			c.AbortWithStatus(401)
		}
	} else {
		c.AbortWithStatus(404)		
	}
	db.WriteData(&data, DATA_PATH)
}

func addGauge(c *gin.Context){
	loc, err := time.LoadLocation(conf.Timezone)
	if err != nil {
		fmt.Println(err)
		return
	}

	var reqGauge gauge
	if err := c.ShouldBind(&reqGauge); err != nil {
		fmt.Println(err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !reqGauge.Validate(){
		fmt.Println("val err")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	err = db.AddGauge(db.Gauge{
		Name: reqGauge.Name,
		Value: 0,
		LastIncrease: time.Now().In(loc).Format(TIME_FORMAT),
	})
	if err != nil{
		fmt.Println(err)
	}
}

func removeGauge(c *gin.Context){
	data, err := db.LoadData(DATA_PATH)
	if err != nil {
		fmt.Println(err)
		return
	}

	var reqGauge gauge
	if err := c.ShouldBind(&reqGauge); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !reqGauge.Validate(){
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if i, exists := findGauge(data.Gauges, reqGauge.Name); exists == true{
		copy(data.Gauges[i:], data.Gauges[i+1:])
		data.Gauges[len(data.Gauges)-1] = db.Gauge{}
		data.Gauges = data.Gauges[:len(data.Gauges)-1]
	} else {
		c.AbortWithStatus(404)		
	}
	db.WriteData(&data, DATA_PATH)
}


func dailyCycle(c *gin.Context){
	data, err := db.LoadData(DATA_PATH)
	if err != nil {
		log.Print()
	}

	for i, e := range data.Gauges {
		if isToday(e.LastIncrease) {
			continue
		}
		data.Gauges[i].Value -= conf.DecreaseStep
		if data.Gauges[i].Value < 0 {
			data.Gauges[i].Value = 0
		}
	}
	db.WriteData(&data, DATA_PATH)
}

func findGauge(g []db.Gauge, name string) (int,bool){
	for i, e := range g {
		if e.Name == name {
			return i, true
		}
	}
	return 0, false
}

func increase(g *db.Gauge) error {
	loc, err := time.LoadLocation(conf.Timezone)
	if err != nil {
		return err
	}


	if isToday(g.LastIncrease){
		return errors.New("Forbidden")
	}

	g.LastIncrease = time.Now().In(loc).Format(TIME_FORMAT)

	if g.Value == conf.MaxValue{
		return nil
	}
	g.Value += conf.IncreaseStep 
	return nil
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
