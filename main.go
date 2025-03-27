package welcomepageapi

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	_ "time/tzdata"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/xklaky35/wpFileReader"
)


const TIME_FORMAT string = time.RFC3339
const LOG_PATH string = "/welcome-page-api/welcomePage.log"
const DATA_PATH string = "/welcome-page-api/data.json"
const CONFIG_PATH string = "/welcome-page-api/config.json"

var config filereader.Config
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
func Init(r *gin.RouterGroup) error {
	err := godotenv.Load()
	if err != nil {
		return err
	}

	config, err = filereader.LoadConfig(CONFIG_PATH)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(LOG_PATH, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)


	r.GET("/GetData", getData)
	r.POST("/UpdateGauge", update) //param
	r.POST("/AddGauge", addGauge) //body
	r.POST("/RemoveGauge", removeGauge) //body
	r.POST("/DailyCycle", dailyCycle)

	return nil
}

func getData(c *gin.Context){
	data, err := filereader.LoadData(DATA_PATH)
	if err != nil {
		fmt.Println(err)
		return
	}

	c.JSON(200, &data)
}

func update(c *gin.Context){
	data, err := filereader.LoadData(DATA_PATH)
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
	filereader.WriteData(&data, DATA_PATH)
}

func addGauge(c *gin.Context){
	data, err := filereader.LoadData(DATA_PATH)
	if err != nil {
		fmt.Println(err)
		return
	}
	loc, err := time.LoadLocation(config.Timezone)
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


	if _, exists := findGauge(data.Gauges, reqGauge.Name); exists == true{
		c.AbortWithStatus(404)		
	} else {
		data.Gauges = append(data.Gauges, filereader.Gauge{
			Name: reqGauge.Name,
			Value: 0,
			LastIncrease: time.Now().In(loc).Format(TIME_FORMAT),
		})
	}

	filereader.WriteData(&data, DATA_PATH)
}

func removeGauge(c *gin.Context){
	data, err := filereader.LoadData(DATA_PATH)
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
		data.Gauges[len(data.Gauges)-1] = filereader.Gauge{}
		data.Gauges = data.Gauges[:len(data.Gauges)-1]
	} else {
		c.AbortWithStatus(404)		
	}
	filereader.WriteData(&data, DATA_PATH)
}


func dailyCycle(c *gin.Context){
	data, err := filereader.LoadData(DATA_PATH)
	if err != nil {
		log.Print()
	}

	for i, e := range data.Gauges {
		if isToday(e.LastIncrease) {
			continue
		}
		data.Gauges[i].Value -= config.DecreaseStep
		if data.Gauges[i].Value < 0 {
			data.Gauges[i].Value = 0
		}
	}
	filereader.WriteData(&data, DATA_PATH)
}

func findGauge(g []filereader.Gauge, name string) (int,bool){
	for i, e := range g {
		if e.Name == name {
			return i, true
		}
	}
	return 0, false
}

func increase(g *filereader.Gauge) error {
	loc, err := time.LoadLocation(config.Timezone)
	if err != nil {
		return err
	}


	if isToday(g.LastIncrease){
		return errors.New("Forbidden")
	}

	g.LastIncrease = time.Now().In(loc).Format(TIME_FORMAT)

	if g.Value == config.MaxValue{
		return nil
	}
	g.Value += config.IncreaseStep 
	return nil
}

func isToday(date string) bool{
	t, err := time.Parse(TIME_FORMAT,date)		
	if err != nil {
		log.Print(err)	
	}
	loc, err := time.LoadLocation(config.Timezone)
	if err != nil {
		log.Print(err)
	}

	if t.Day() != time.Now().In(loc).Day(){
		return false
	}
	return true
}
