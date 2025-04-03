package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/mattn/go-sqlite3"
)

const DB_PATH string = "./local/db.db"

type Data struct {
	Gauges []Gauge `json:"gauges"`
}

type Gauge struct {
	Name string `json:"name"`
	Value int `json:"value"`
	LastIncrease string `json:"last_increase"`
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


func LoadData(path string) (Data, error){
	var data Data  
	f, err := os.ReadFile(path)
	if err != nil {
		return data, err
	}

	err = json.Unmarshal(f, &data)

	return data, nil
}


func WriteData(d *Data, path string) error {

	data, err := json.Marshal(d)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, data, 0666)
	if err != nil{
		return err
	}
	return nil
}


