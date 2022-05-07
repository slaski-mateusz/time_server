package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"
)

const (
	MIN_TCP_PORT = 0
	MAX_TCP_PORT = 65535
)

type DateInfo struct {
	Year  int `json:"year,omitempty"`
	Month int `json:"month,omitempty"`
	Day   int `json:"day,omitempty"`
}

type TimeInfo struct {
	Hour       int     `json:"hour,omitempty"`
	Minute     int     `json:"minute,omitempty"`
	Second     int     `json:"second,omitempty"`
	NanoSecond float32 `json:"nano_second,omitempty"`
}

type TimeZoneInfo struct {
	Name  string `json:"name,omitempty"`
	Shift int    `json:"shift,omitempty"`
}

type DateTimeInfo struct {
	Datedata *DateInfo     `json:"date,omitempty"`
	Timedata *TimeInfo     `json:"time,omitempty"`
	Tzdata   *TimeZoneInfo `json:"tz,omitempty"`
}

type Configuration struct {
	Logging struct {
		FileName string `yaml:"file_name,omitempty"`
		Unit     string `yaml:"unit,omitempty"`
		Size     uint   `yaml:"size,omitempty"`
		Files    uint   `yaml:"files,omitempty"`
	}
	Web struct {
		NetIntf string `yaml:"netintf,omitempty"`
		Port    uint   `yaml:"port,omitempty"`
	}
}

type ErrorMessage struct {
	ErrorMessage string `json:"error_message"`
}

type ApiNode struct {
	nodeName string
	function func(resWri http.ResponseWriter, requ *http.Request)
	children []*ApiNode
}

var api_structure = &ApiNode{
	nodeName: "",
	function: doc_page,
	children: []*ApiNode{
		{
			nodeName: "now",
			function: http.NotFound,
			children: []*ApiNode{
				{
					nodeName: "iso",
					function: iso_datetime,
				},
				{
					nodeName: "unix",
					function: unix_timestamp,
				},
				{
					nodeName: "parsed",
					function: datetime_parsed,
				},
			},
		},
		{
			nodeName: "convert",
			function: http.NotFound,
			children: []*ApiNode{
				{
					nodeName: "timezone",
					function: convert_timezone,
				},
				{
					nodeName: "listtimezones",
					function: list_timezones,
				},
			},
		},
	},
}

type OutDatetimeData struct {
	TimeZone       string
	DateTimeString string
}

type InDatetimeData struct {
	FromTimezone   string
	ToTimezone     string
	DatetimeString string
}

func Contains[T comparable](sl []T, el T) bool {
	for _, val := range sl {
		if val == el {
			return true
		}
	}
	return false
}

func doc_page(resWri http.ResponseWriter, requ *http.Request) {
	docData, _ := os.ReadFile("documentation.html")
	fmt.Fprintf(resWri, string(docData))
}

func wrong_timezone_message(tz_name string) string {
	return fmt.Sprintf("Wrong timezone name given '%v' please use /convert/listtimezones endpoint to get list of valid timezones", tz_name)
}

func iso_datetime(resWri http.ResponseWriter, requ *http.Request) {
	urlVars := requ.URL.Query()
	outTz := urlVars.Get("outtz")
	if outTz == "" {
		outTz = "UTC"
	}
	outLocation, outTzErr := time.LoadLocation(outTz)
	if outTzErr == nil {
		isoTime := fmt.Sprintf("%v", time.Now().In(outLocation))
		outData := map[string]string{"iso_datetime": isoTime}
		json.NewEncoder(resWri).Encode(outData)
	} else {
		errorMessage := wrong_timezone_message(outTz)
		var outData ErrorMessage = ErrorMessage{
			ErrorMessage: errorMessage,
		}
		json.NewEncoder(resWri).Encode(outData)
	}
}

func unix_timestamp(resWri http.ResponseWriter, requ *http.Request) {
	unixTs := time.Now().Unix()
	outData := map[string]int64{"unix_timestamp": unixTs}
	json.NewEncoder(resWri).Encode(outData)
}

func check_argument(ok_values []string, arg_to_check string) bool {
	if slices.Contains(ok_values, arg_to_check) {
		return true
	}
	return false
}

func load_timezones() []string {
	tzData, _ := os.ReadFile("timezones.dat")
	return strings.Split(string(tzData), "\n")
}

func datetime_parsed(resWri http.ResponseWriter, requ *http.Request) {
	urlVars := requ.URL.Query()
	sendMarkers := []string{"1", "yes", "on", "true"}
	dateReq := strings.ToLower(urlVars.Get("date"))
	sendDate := check_argument(sendMarkers, dateReq)
	timeReq := strings.ToLower(urlVars.Get("time"))
	sendTime := check_argument(sendMarkers, timeReq)
	tzReq := strings.ToLower(urlVars.Get("tz"))
	sendTz := check_argument(sendMarkers, tzReq)
	outTz := urlVars.Get("outtz")
	outLocation, outTzErr := time.LoadLocation(outTz)
	if outTzErr == nil {
		// if no query argument than default behaviour is to send all data
		if !sendDate && !sendTime && !sendTz {
			sendDate = true
			sendTime = true
			sendTz = true
		}
		timeNow := time.Now().In(outLocation)
		tzName, tzShift := timeNow.Zone()
		outData := DateTimeInfo{}
		if sendDate {
			outData.Datedata = &DateInfo{
				Year:  timeNow.Year(),
				Month: int(timeNow.Month()),
				Day:   timeNow.Day(),
			}
		}
		if sendTime {
			outData.Timedata = &TimeInfo{
				Hour:       timeNow.Hour(),
				Minute:     timeNow.Minute(),
				Second:     timeNow.Second(),
				NanoSecond: float32(timeNow.Nanosecond()),
			}
		}
		if sendTz {
			outData.Tzdata = &TimeZoneInfo{
				Name:  tzName,
				Shift: tzShift,
			}
		}
		json.NewEncoder(resWri).Encode(outData)
	} else {
		var outData ErrorMessage
		errMessage := wrong_timezone_message(outTz)
		outData.ErrorMessage = errMessage
		json.NewEncoder(resWri).Encode(outData)
	}
}

func list_timezones(resWri http.ResponseWriter, requ *http.Request) {
	tzList := load_timezones()
	json.NewEncoder(resWri).Encode(tzList)
}

func convert_timezone(resWri http.ResponseWriter, requ *http.Request) {
	var inputDatetime InDatetimeData
	var outputDatetime OutDatetimeData
	datetimeLayout := "2006-01-02T15:04:05"
	decErr := json.NewDecoder(requ.Body).Decode(&inputDatetime)
	if decErr != nil {
		http.Error(resWri, decErr.Error(), http.StatusBadRequest)
		return
	}
	fromLocation, fLocErr := time.LoadLocation(inputDatetime.FromTimezone)
	if fLocErr != nil {
		http.Error(resWri, fLocErr.Error(), http.StatusBadRequest)
		return
	}
	toLocation, tLocErr := time.LoadLocation(inputDatetime.ToTimezone)
	if tLocErr != nil {
		http.Error(resWri, tLocErr.Error(), http.StatusBadRequest)
		return
	}
	dateTimeToConvert, parseErr := time.ParseInLocation(
		datetimeLayout,
		inputDatetime.DatetimeString,
		fromLocation,
	)
	if parseErr != nil {
		fmt.Println(parseErr)
		http.Error(resWri, parseErr.Error(), http.StatusBadRequest)
		return
	}
	convertedDatetime := dateTimeToConvert.In(toLocation)
	outputDatetime.DateTimeString = convertedDatetime.Format(datetimeLayout)
	outputDatetime.TimeZone = inputDatetime.ToTimezone
	json.NewEncoder(resWri).Encode(outputDatetime)

}

var router = mux.NewRouter().StrictSlash(true)

func activate_api_node(in_uri string, node *ApiNode) {
	var nodeUri string
	if node.nodeName == "" {
		nodeUri = "/"
	} else {
		nodeUri = fmt.Sprintf("%v/", node.nodeName)
	}
	apiUri := in_uri + nodeUri
	if node.function != nil {
		router.HandleFunc(apiUri, node.function)
	}
	for _, child := range node.children {
		activate_api_node(apiUri, child)
	}
}

func handle_requests(net_intf string, net_port uint) {
	activate_api_node("", api_structure)
	webIntf := fmt.Sprintf("%v:%v", net_intf, net_port)
	log.Fatal(http.ListenAndServe(webIntf, router))
}

func default_configuration() Configuration {
	var returnConf Configuration
	returnConf.Logging.FileName = "tserver.log"
	returnConf.Logging.Unit = "k"
	returnConf.Logging.Size = 100
	returnConf.Logging.Files = 10
	returnConf.Web.NetIntf = "127.0.0.1"
	returnConf.Web.Port = 8888
	return returnConf
}

func print_configuration(cnf Configuration) {
	cnfBy, _ := yaml.Marshal(cnf)
	fmt.Println(string(cnfBy))
}

func valid_configuration(ctv Configuration) (bool, error) {
	var availableUnits = []string{"M", "k"}
	confValid := true
	var errMessages []string
	if !Contains(availableUnits, ctv.Logging.Unit) {
		confValid = false
		errMessages = append(
			errMessages,
			fmt.Sprintf(
				"Logging configuration file size unit '%v' is not allowed unit: %v",
				ctv.Logging.Unit,
				strings.Join(availableUnits, ", "),
			),
		)
	}
	if ctv.Logging.Files < 1 {
		confValid = false
		errMessages = append(
			errMessages,
			fmt.Sprintf(
				"Provided number of log files '%v' must be minimum 1.",
			),
		)
	}
	if ctv.Web.Port < MIN_TCP_PORT || ctv.Web.Port > MAX_TCP_PORT {
		confValid = false
		errMessages = append(
			errMessages,
			fmt.Sprintf(
				"Configured API port %v is not in allowed range from %v to %v",
				ctv.Web.Port,
				MIN_TCP_PORT,
				MAX_TCP_PORT,
			),
		)
	}
	int_addr_splitted := strings.Split(ctv.Web.NetIntf, ".")
	if len(int_addr_splitted) != 4 {
		confValid = false
		errMessages = append(
			errMessages,
			fmt.Sprintf(
				"Interface IP address '%v' is not A.B.C.D pattern",
				ctv.Web.NetIntf,
			),
		)
	} else {
		for oi, octet := range int_addr_splitted {
			octint, stiErr := strconv.ParseInt(octet, 0, 8)
			if stiErr != nil {
				confValid = false
				errMessages = append(
					errMessages,
					fmt.Sprintf(
						"%v octet '%v' of address is not integer value",
						oi,
						octet,
					),
				)
			} else {
				if octint < 0 || octint > 254 {
					confValid = false
					errMessages = append(
						errMessages,
						fmt.Sprintf(
							"%v octet '%v' of address is out of range 0~254",
							oi,
							octet,
						),
					)
				}
			}
		}
	}
	return confValid, errors.New(strings.Join(errMessages, "\n"))
}

func main() {
	defConfFilename := "config.yaml"
	configFileName := flag.String(
		"conf_file",
		"",
		"Configuration file name",
	)
	flag.Parse()
	if *configFileName == "" {
		fmt.Println("Configuration filename not given as command line parameter. Trying use file config.yaml.")
		configFileName = &defConfFilename
	}
	configInFile, readErr := os.ReadFile(*configFileName)
	var config Configuration
	var configFromFile Configuration
	var configToCheck Configuration
	if readErr == nil {
		fmt.Printf("Able to read file '%s'\n", *configFileName)
		unmErr := yaml.Unmarshal(configInFile, &configFromFile)
		if unmErr == nil {
			fmt.Printf("Able to parse file '%s' content\n", *configFileName)
			configToCheck = configFromFile
		} else {
			fmt.Printf(
				"Problem with parsing YAML in configuration file '%v': '%v'\n",
				*configFileName,
				unmErr,
			)
			fmt.Println("File content:")
			fmt.Println(string(configInFile))
		}
	} else {
		fmt.Printf(
			"Problem with reading configuration file '%v': '%v'\n",
			*configFileName,
			readErr,
		)
	}
	if configToCheck != (Configuration{}) {
		if cv, cvErr := valid_configuration(configToCheck); cv {
			config = configToCheck
		} else {
			fmt.Printf(
				"Configuration file content '%v' issue: '%v'\n",
				*configFileName,
				cvErr,
			)

		}
	}
	if config == (Configuration{}) {
		config = default_configuration()
		fmt.Println("Not able to run with provided configuration.")
		fmt.Println("Starting with below default buildin configuration")
	}
	print_configuration(config)
	handle_requests(config.Web.NetIntf, config.Web.Port)
}
