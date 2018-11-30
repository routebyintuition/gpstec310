//Unicorn Rentals Reservation Lookup Routine

package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

import _ "github.com/go-sql-driver/mysql"

func databaseTableExists(sc serviceConfig, db *sql.DB) (int, bool) {
	entryCount := 0
	exists := true
	queryTable := "SELECT count(*) FROM "
	queryTable += sc.MysqlTable
	queryTable += " as CNT"
	err := db.QueryRow(queryTable).Scan(&entryCount)
	if err != nil {
		log.Printf("Database: %v Table: %v does not exist", sc.MysqlDb, sc.MysqlTable)
		return entryCount, false
	} else {
		log.Printf("Database: %v Table: %v has %v entries", sc.MysqlDb, sc.MysqlTable, entryCount)
	}

	return entryCount, exists
}

func createTable(sc serviceConfig, db *sql.DB) error {
	log.Println("Table does not exist...creating....")
	insertQuery := "CREATE TABLE " + sc.MysqlTable
	insertQuery += " (id int(11) NOT NULL PRIMARY KEY AUTO_INCREMENT, reservationid varchar(255), "
	insertQuery += "startdatetime varchar(255), enddatetime varchar(255), "
	insertQuery += "pickuplocation varchar(255), reservationtitle varchar(255), "
	insertQuery += "accesscode varchar(255) )"
	_, err := db.Exec(insertQuery)
	if err != nil {
		log.Printf("Could not create table: %v error: %v", sc.MysqlTable, err)
		return err
	}
	return nil
}

func databaseWrite(sc serviceConfig, db *sql.DB) (string, error) {

	// Using the download URL from SSM parameter store with the variable name
	// we got from CloudFormation template, download the content of the Unicorn
	// Rentals reservation database in CSV format.
	log.Printf("Downloading reservations data from: %v URL", sc.DownloadUrl)
	resp, err := http.Get(sc.DownloadUrl)
	if err != nil {
		log.Println("Could not open URL to downlaod data: ", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Here we read in the content of the file for CSV parsing
	// Lambda is a read-only file system so we have to do this in memory
	reader := csv.NewReader(resp.Body)

	rowCount := 0

	entryCount, tableExists := databaseTableExists(sc, db)
	if !tableExists {
		log.Println("Could not create table.")
		os.Exit(1)
	}

	for {
		line, error := reader.Read()
		if error == io.EOF {
			break
		} else if error != nil {
			log.Fatal(error)
		}
		rowCount++
		insertStatement := "INSERT INTO reservations (reservationid, startdatetime, enddatetime, pickuplocation, reservationtitle, accesscode) values (?,?,?,?,?,?)"
		_, err = db.Exec(insertStatement, line[0], line[1], line[2], line[3], line[4], line[5])
		if err != nil {
			log.Println("MySQL Insert Error: ", err)
			os.Exit(1)
		}
	}
	entryCount = entryCount + rowCount
	return fmt.Sprintf("Completed %v rows for a total of %v rows", rowCount, entryCount), nil
}
