package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"
)

type DateInfo struct {
	Year  int `json:"year,omitempty"`
	Month int `json:"month,omitempty"`
	Day   int `json:"day,omitempty"`
}

type TimeInfo struct {
	Hour        int     `json:"hour,omitempty"`
	Minute      int     `json:"minute,omitempty"`
	Second      int     `json:"second,omitempty"`
	Nano_second float32 `json:"nano_second,omitempty"`
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
		File_name string `yaml:"file_name,omitempty"`
		Unit      string `yaml:"unit,omitempty"`
		Size      int    `yaml:"size,omitempty"`
		Files     int    `yaml:"files,omitempty"`
	}
	Web struct {
		Netintf string `yaml:"netintf,omitempty"`
		Port    int    `yaml:"port,omitempty"`
	}
}

type ApiNode struct {
	node_name string
	function  func(res_wri http.ResponseWriter, requ *http.Request)
	children  []*ApiNode
}

var api_structure = &ApiNode{
	node_name: "",
	function:  doc_page,
	children: []*ApiNode{
		&ApiNode{
			node_name: "now",
			function:  not_supported,
			children: []*ApiNode{
				&ApiNode{
					node_name: "iso",
					function:  iso_datetime,
				},
				&ApiNode{
					node_name: "unix",
					function:  unix_timestamp,
				},
				&ApiNode{
					node_name: "parsed",
					function:  datetime_parsed,
				},
			},
		},
		&ApiNode{
			node_name: "convert",
			function:  not_supported,
			children: []*ApiNode{
				&ApiNode{
					node_name: "timezone",
					function:  convert_timezone,
				},
				&ApiNode{
					node_name: "format",
					function:  convert_format,
				},
			},
		},
	},
}

func doc_page(res_wri http.ResponseWriter, requ *http.Request) {
	welcome_message := `This is demo API time server.
Use the following endpoints for GET method:
- /now/iso to get ISO formated datetime
    query argument "outtz" allow to define time zone of received time data
	default is return UTC
- /now/unix to get unix timestamp
    no query arguments are handled unix epoch is given as gmt
- /now/parsed to receive all values separatelly in dictionary
	query argument "outtz" allow to define time zone of received time data
	query argument "date" limit response to date
	query argument time limit response to time
	query argument "tz" limit response to time zone
	all arguments may be mixed no date/time/tz result default output of full datetime info
Use the following endpoint for POST method:
- /convert
    JSON data
	{
		"from_timestamp": ISO/UNIX_TIMESTAMP,
		"to_format": "ISO/UNIX_TIMESTAMP,
		"to_tz"
    }
`
	fmt.Fprintf(res_wri, welcome_message)
}

func not_supported(res_wri http.ResponseWriter, requ *http.Request) {
	fmt.Fprintf(res_wri, "404 page not found")
}

func iso_datetime(res_wri http.ResponseWriter, requ *http.Request) {
	iso_time := fmt.Sprintf("%v", time.Now())
	out_data := map[string]string{"iso_datetime": iso_time}
	output, _ := json.Marshal(out_data)
	fmt.Fprintf(res_wri, string(output))
}

func unix_timestamp(res_wri http.ResponseWriter, requ *http.Request) {
	unix_ts := time.Now().Unix()
	out_data := map[string]int64{"unix_timestamp": unix_ts}
	output, _ := json.Marshal(out_data)
	fmt.Fprintf(res_wri, string(output))
}

func check_argument(ok_values []string, arg_to_check string) bool {
	if slices.Contains(ok_values, arg_to_check) {
		return true
	}
	return false
}

func datetime_parsed(res_wri http.ResponseWriter, requ *http.Request) {
	url_vars := requ.URL.Query()
	send_markers := []string{"1", "yes", "on", "true"}
	date_req := strings.ToLower(url_vars.Get("date"))
	send_date := check_argument(send_markers, date_req)
	time_req := strings.ToLower(url_vars.Get("time"))
	send_time := check_argument(send_markers, time_req)
	tz_req := strings.ToLower(url_vars.Get("tz"))
	send_tz := check_argument(send_markers, tz_req)
	// if no query argument than default behaviour is to send all data
	if !send_date && !send_time && !send_tz {
		send_date = true
		send_time = true
		send_tz = true
	}
	time_now := time.Now()
	tz_name, tz_shift := time_now.Zone()
	out_data := DateTimeInfo{}
	if send_date {
		out_data.Datedata = &DateInfo{
			Year:  time_now.Year(),
			Month: int(time_now.Month()),
			Day:   time_now.Day(),
		}
	}
	if send_time {
		out_data.Timedata = &TimeInfo{
			Hour:        time_now.Hour(),
			Minute:      time_now.Minute(),
			Second:      time_now.Second(),
			Nano_second: float32(time_now.Nanosecond()),
		}
	}
	if send_tz {
		out_data.Tzdata = &TimeZoneInfo{
			Name:  tz_name,
			Shift: tz_shift,
		}
	}
	output, err := json.Marshal(out_data)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Fprintf(res_wri, string(output))
}

func convert_timezone(res_wri http.ResponseWriter, requ *http.Request) {
	fmt.Fprintf(res_wri, "Convert activated")
}

func convert_format(res_wri http.ResponseWriter, requ *http.Request) {
	fmt.Fprintf(res_wri, "Convert activated")
}

var router = mux.NewRouter().StrictSlash(true)

func activate_api_node(in_uri string, node *ApiNode) {
	var node_uri string
	if node.node_name == "" {
		node_uri = "/"
	} else {
		node_uri = fmt.Sprintf("%v/", node.node_name)
	}
	api_uri := in_uri + node_uri
	if node.function != nil {
		router.HandleFunc(api_uri, node.function)
	}
	for _, child := range node.children {
		activate_api_node(api_uri, child)
	}
}

func handle_requests(net_intf string, net_port int) {
	activate_api_node("", api_structure)
	web_intf := fmt.Sprintf("%v:%v", net_intf, net_port)
	log.Fatal(http.ListenAndServe(web_intf, router))
}

func main() {
	config_filename := "config.yaml"
	var config Configuration
	config.Logging.File_name = "tserver.log"
	config.Logging.Unit = "k"
	config.Logging.Size = 100
	config.Logging.Files = 10
	config.Web.Netintf = "0.0.0.0"
	config.Web.Port = 8888
	config_in_file, read_err := os.ReadFile(config_filename)
	var config_from_file Configuration
	if read_err == nil {
		unm_err := yaml.Unmarshal(config_in_file, &config_from_file)
		if unm_err == nil {
			config = config_from_file
		}
	}
	handle_requests(config.Web.Netintf, config.Web.Port)
}
