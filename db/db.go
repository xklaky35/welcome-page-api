package db

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/mattn/go-sqlite3"
)

const DB_PATH string = "./local/db.db"

type Data struct {
	Gauges []Gauge
}

type Gauge struct {
	Name string 
	Value int 
	LastIncrease string 
	GaugeId int
}

func Init() {
	if !slices.Contains(sql.Drivers(), "sqlite3"){
		sql.Register("sqlite3", &sqlite3.SQLiteDriver{})
	}
}


func AddGauge(gauge Gauge) error{
	db, err := sql.Open("sqlite3", DB_PATH)
	if err != nil{
		fmt.Println(err)
		return err
	}
	defer db.Close()

	
	trans, err := db.Begin()
	_, err = trans.Exec(fmt.Sprintf("INSERT INTO gauges (name) VALUES ('%s');", gauge.Name))
	if err != nil{
		trans.Rollback()
		return err
	}

	// we need the id of the gauge first before we can add the data to the database
	var gauge_id int
	err = trans.QueryRow(fmt.Sprintf("SELECT id FROM gauges WHERE name='%s';", gauge.Name)).Scan(&gauge_id)
	if err != nil{
		trans.Rollback()
		return err
	}

	_, err = trans.Exec(fmt.Sprintf("INSERT INTO data (value, timestamp, gauge_id) VALUES(%d, '%s', %d);", gauge.Value, gauge.LastIncrease, gauge_id))
	if err != nil{
		trans.Rollback()
		return err
	}

	trans.Commit()
	return nil
}


func LoadData() (Data, error){
	var data Data  

	db, err := sql.Open("sqlite3", DB_PATH)
	if err != nil{
		fmt.Println(err)
		return data, err
	}
	defer db.Close()

	// query not finished yet
	// i need a way to get the updated value after a daily cicle. The timestamp does not change on this event
	rows, err := db.Query("SELECT name, value, MAX(timestamp), gauge_id FROM gauges g JOIN data d ON g.id = d.gauge_id GROUP BY name;")
	if err != nil {
		return data, err
	}

	var gName, gTimestamp string
	var gValue,gId int

	for rows.Next() {
		rows.Scan(&gName, &gValue, &gTimestamp, &gId)
		data.Gauges = append(data.Gauges, Gauge{
			Name: gName,
			Value: gValue,
			LastIncrease: gTimestamp,
			GaugeId: gId,
		})
	}
	return data, nil
}


func UpdateGauge(name string, timestamp string, increase int, min int, max int ) error {
	db, err := sql.Open("sqlite3", DB_PATH)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer db.Close()

	gauge := GetGauge(name)
	
	newVal := gauge.Value + increase

	if newVal > max {
		newVal = max
	}
	if newVal < min {
		newVal = min
	}

	query := fmt.Sprintf("INSERT INTO data (value, timestamp, gauge_id) VALUES (%d, '%s', %d)", newVal, timestamp,  gauge.GaugeId)

	_, err = db.Exec(query)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func GetGauge(name string) Gauge {
	var gauge Gauge
	data, err := LoadData()
	if err != nil {
		return gauge
	}

	for _, e := range(data.Gauges) {
		if name == e.Name {
			gauge = e
		}
	}
	return gauge
}

func RemoveGauge(name string) error {
	db, err := sql.Open("sqlite3", DB_PATH)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer db.Close()

	gauge := GetGauge(name)
	
	tran, err := db.Begin()
	if err != nil {
		return errors.New("Failed to start transaction!")
	}
	delGauges := fmt.Sprintf("DELETE FROM gauges WHERE name = '%s'", name)
	delData := fmt.Sprintf("DELETE FROM data WHERE gauge_id = '%d'", gauge.GaugeId)

	tran.Exec(delGauges)
	tran.Exec(delData)

	err = tran.Commit()
	if err != nil {
		return err
	}

	return nil
}
