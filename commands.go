//Unicorn Rentals Reservation Lookup Routine

package main

import (
	"database/sql"
	"fmt"
	"log"
)

import _ "github.com/go-sql-driver/mysql"

func dropTable(sc serviceConfig, db *sql.DB) error {
	dropQuery := "DROP TABLE " + sc.MysqlTable
	_, err := db.Exec(dropQuery)
	if err != nil {
		log.Println("Could not drop table: ", err)
		return err
	}
	return nil
}

func deleteTable(sc serviceConfig, db *sql.DB) error {
	delQuery := "delete from " + sc.MysqlTable
	_, err := db.Exec(delQuery)
	if err != nil {
		log.Println("Could not delete from table: %v, database: %v, error: %v", sc.MysqlTable, sc.MysqlDb, err)
		return err
	}
	return nil
}

func doCommand(reqInfo ReqInfo, sc serviceConfig, db *sql.DB) error {
	switch reqInfo.Command {
	case "droptable":
		log.Printf("Recieved command to drop table: %v, database: %v", sc.MysqlTable, sc.MysqlDb)
		err := dropTable(sc, db)
		if err != nil {
			log.Println("Error on dropping table: ", err)
			return err
		}
	case "deletetable":
		log.Printf("Recieved command to delete from table: %v, database: %v", sc.MysqlTable, sc.MysqlDb)
		err := deleteTable(sc, db)
		if err != nil {
			log.Println("Error on dropping table: ", err)
			return err
		}
	case "inserttable":
		log.Printf("Recieved command to insert new data from %v into table: %v, database: %v", sc.DownloadUrl, sc.MysqlTable, sc.MysqlDb)
		insertRes, err := databaseWrite(sc, db)
		if err != nil {
			log.Println("Error on dropping table: ", err)
			return err
		}
		log.Println("Insert command results: ", insertRes)
	default:
		// Unknown command
		fmt.Println("Unknown command: ", reqInfo.Command)
	}

	return nil
}
