//Unicorn Rentals Reservation Lookup Routine

package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"log"
	"net/http"
	"os"
)

import _ "github.com/go-sql-driver/mysql"

// Data structure holding our service configuration information
type serviceConfig struct {
	MysqlHost   string
	MysqlPort   string
	MysqlUser   string
	MysqlPass   string
	MysqlDb     string
	MysqlTable  string
	DownloadUrl string
}

// Data structure holding details for each reservation
type ReservationDetails struct {
	ConfirmationCode      string `json:ConfirmationCode`
	ReservationStartDate  string `json:ReservationStartDate`
	ReservationEndDate    string `json:ReservationEndDate`
	UnicornAccessCode     string `json:UnicornAccessCode`
	UnicornPickupLocation string `json:UnicornPickupLocation`
	UnicornResTitle       string `json:UnicornResTitle`
}

// Data structure used to parse the incoming request
// - Test - contains testing/debugging information
// - Resid - contains the reservation ID we will be looking up
type ReqInfo struct {
	Test    string `json:test`
	Resid   string `json:resid`
	Command string `json:command`
}

var ssmsvc *ssm.SSM

func serverError(err error) (events.APIGatewayProxyResponse, error) {
	log.Println(err.Error())

	respEvent := events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       http.StatusText(http.StatusInternalServerError),
		Headers:    make(map[string]string),
	}
	respEvent.Headers["Access-Control-Allow-Origin"] = "*"
	return respEvent, nil
}

func clientError(status int) (events.APIGatewayProxyResponse, error) {
	respEvent := events.APIGatewayProxyResponse{
		StatusCode: status,
		Body:       http.StatusText(status),
		Headers:    make(map[string]string),
	}
	respEvent.Headers["Access-Control-Allow-Origin"] = "*"

	return respEvent, nil
}

func checkAndGetSsm(varName string, ssmsvc *ssm.SSM) string {
	varValue, _ := checkEnvLen(os.LookupEnv(varName))
	log.Print("Getting value of SSM parameter: ", varName)
	ssmValue := getSsmParam(varValue, varName, ssmsvc)

	return ssmValue
}

func checkEnvLen(mvar string, exists bool) (string, bool) {
	if !exists {
		log.Fatal("Lambda ENV variables not set.")
		//os.Exit(1)
	}
	varLen := len(mvar)
	if varLen < 1 {
		return "EMPTY", false
	}

	return mvar, exists
}

func getSsmParam(name string, desc string, ssmsvc *ssm.SSM) string {
	param, err := ssmsvc.GetParameter(&ssm.GetParameterInput{
		Name: &name,
	})
	if err != nil {
		log.Fatal("Could not get SSM parameter: %v of ENV variable: %v", name, err)
	}

	value := *param.Parameter.Value

	if len(value) < 1 {
		log.Fatal("Could not get SSM parameter: %v of ENV variable: %v", name, desc)
		os.Exit(1)
	}

	return value
}

func databaseLookup(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	var reqInfo ReqInfo

	decoded, errJSON := base64.StdEncoding.DecodeString(req.Body)

	if errJSON != nil {
		log.Printf("Req is not base64 encoded: %v ", req.Body)
		decoded = []byte(req.Body)
	} else {
		log.Print("Decoded String: ", string(decoded))
	}

	errJ := json.Unmarshal(decoded, &reqInfo)
	if errJ != nil {
		log.Print("Cannot unmarshall JSON: ", errJ)
		return clientError(http.StatusUnprocessableEntity)
	}

	if reqInfo.Resid == "" {
		log.Print("No reservation ID provided: ", reqInfo)
		return clientError(http.StatusBadRequest)
	}

	log.Print("Full Request: ", reqInfo)

	//Retrieve SSM parameter names for database connection from ENV variables
	var sc serviceConfig

	awsRegion, exists := checkEnvLen(os.LookupEnv("AWS_REGION"))
	if !exists {
		log.Print("AWS Region ENV variables not set.")
		os.Exit(1)
	}
	log.Printf("Pulling SSM params via AWS Region: %v", awsRegion)

	awscfg := &aws.Config{}
	awscfg.WithRegion(awsRegion)
	sess := session.Must(session.NewSession(awscfg))
	ssmsvc = ssm.New(sess, awscfg)

	log.Print("Initialized AWS connection to SSM")

	sc.MysqlUser = checkAndGetSsm("UNICORN_MYSQLUSER", ssmsvc)
	sc.MysqlPass = checkAndGetSsm("UNICORN_MYSQLPASS", ssmsvc)
	sc.MysqlHost = checkAndGetSsm("UNICORN_MYSQLHOST", ssmsvc)
	sc.MysqlPort = checkAndGetSsm("UNICORN_MYSQLPORT", ssmsvc)
	sc.MysqlDb = checkAndGetSsm("UNICORN_MYSQLDB", ssmsvc)
	sc.MysqlTable = checkAndGetSsm("UNICORN_MYSQLTABLE", ssmsvc)
	sc.DownloadUrl = checkAndGetSsm("DOWNLOAD_URL", ssmsvc)

	log.Print("Lookup ENV setup completed")

	// Unicorn Rentals Reservation System CONFIG setup for MySQL
	var reservationsData ReservationDetails

	//MySQL service for GPS DevOps
	sqlAccessString := sc.MysqlUser + ":" + sc.MysqlPass + "@tcp(" + sc.MysqlHost + ":" + sc.MysqlPort + ")/" + sc.MysqlDb
	rowCount := 0
	var mreservationid string
	var mstartdatetime string
	var menddatetime string
	var mpickuplocation string
	var mreservationtitle string
	var maccesscode string

	db, err := sql.Open("mysql", sqlAccessString)
	if err != nil {
		log.Printf("MySQL Error: %v", err)
	}

	if len(reqInfo.Command) > 1 {
		err := doCommand(reqInfo, sc, db)
		if err != nil {
			log.Printf("Running recieved command: %v and received error: %v", reqInfo.Command, err)
		}
	}

	// Check to make sure that the table exists and if not, create it.
	// When deployed with CloudFormation, the first run will setup the necessary tables
	entryCount, tableExists := databaseTableExists(sc, db)
	if !tableExists {
		errCreate := createTable(sc, db)
		if errCreate != nil {
			log.Printf("Could not create table: %v due to error: %v", sc.MysqlTable, errCreate)
			os.Exit(1)
		} else {
			populateCount, errPopulate := databaseWrite(sc, db)
			if errPopulate != nil {
				log.Println("Error populating reservation data into new table: ", errPopulate)
				os.Exit(1)
			} else {
				log.Printf("Populated %v rows on database server.", populateCount)
			}
		}
	} else {
		if entryCount < 1 {
			populateCount, errPopulate := databaseWrite(sc, db)
			if errPopulate != nil {
				log.Println("Error populating reservation data into new table: ", errPopulate)
				os.Exit(1)
			} else {
				log.Printf("Populated %v rows on database server.", populateCount)
			}
		}
	}

	// Now that we have ensured the table exists and is populated with data, we can query
	queryString := "SELECT reservationid, startdatetime, enddatetime, pickuplocation, reservationtitle, accesscode FROM " + sc.MysqlTable + " where reservationid = ?"
	rows, err := db.Query(queryString, reqInfo.Resid)
	if err != nil {
		log.Printf("MySQL error on SELECT query: %v \n", err)
	}
	defer rows.Close()
	defer db.Close()

	for rows.Next() {
		//if we have at least have one row of results
		rowErr := rows.Scan(&mreservationid, &mstartdatetime, &menddatetime, &mpickuplocation, &mreservationtitle, &maccesscode)
		if rowErr != nil {
			log.Print("Error on lookup: ", mreservationid)
			log.Print("MySQL result row error: ", rowErr)
		}
		rowCount++
	}

	if rowCount > 0 {
		log.Print("Retreived reservation ID: ", mreservationid)

		reservationsData.ConfirmationCode = reqInfo.Resid
		reservationsData.ReservationStartDate = mstartdatetime
		reservationsData.ReservationEndDate = menddatetime
		reservationsData.UnicornAccessCode = maccesscode
		reservationsData.UnicornPickupLocation = mpickuplocation
		reservationsData.UnicornResTitle = mreservationtitle

	} else {
		log.Print("No such reservation: ", reqInfo.Resid)

		reservationsData.ConfirmationCode = reqInfo.Resid
		reservationsData.ReservationStartDate = "EMPTY"
		reservationsData.ReservationEndDate = "EMPTY"
		reservationsData.UnicornAccessCode = "EMPTY"
		reservationsData.UnicornPickupLocation = "EMPTY"
		reservationsData.UnicornResTitle = "EMPTY"

	}

	reservationsDataJs, err := json.Marshal(reservationsData)
	if err != nil {
		log.Print("JSON Marshal error: ", err)
		return serverError(err)
	}

	respEvent := events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(reservationsDataJs),
		Headers:    make(map[string]string),
	}

	respEvent.Headers["Access-Control-Allow-Origin"] = "*"
	return respEvent, nil
}

func main() {
	lambda.Start(databaseLookup)
}
