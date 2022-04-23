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

type ErrorMessage struct {
	Error_message string `json:"error_message"`
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
		{
			node_name: "now",
			function:  http.NotFound,
			children: []*ApiNode{
				{
					node_name: "iso",
					function:  iso_datetime,
				},
				{
					node_name: "unix",
					function:  unix_timestamp,
				},
				{
					node_name: "parsed",
					function:  datetime_parsed,
				},
			},
		},
		{
			node_name: "convert",
			function:  http.NotFound,
			children: []*ApiNode{
				{
					node_name: "timezone",
					function:  convert_timezone,
				},
				{
					node_name: "listtimezones",
					function:  list_timezones,
				},
			},
		},
	},
}

func doc_page(res_wri http.ResponseWriter, requ *http.Request) {
	doc_data, _ := os.ReadFile("documentation.html")
	fmt.Fprintf(res_wri, string(doc_data))
}

func wrong_timezone_message(tz_name string) string {
	return fmt.Sprintf("Wrong timezone name given '%v' please use /convert/listtimezones endpoint to get list of valid timezones", tz_name)
}

func iso_datetime(res_wri http.ResponseWriter, requ *http.Request) {
	url_vars := requ.URL.Query()
	out_tz := url_vars.Get("outtz")
	if out_tz == "" {
		out_tz = "UTC"
	}
	out_location, out_tz_err := time.LoadLocation(out_tz)
	if out_tz_err == nil {
		iso_time := fmt.Sprintf("%v", time.Now().In(out_location))
		out_data := map[string]string{"iso_datetime": iso_time}
		json.NewEncoder(res_wri).Encode(out_data)
	} else {
		error_message := wrong_timezone_message(out_tz)
		var out_data ErrorMessage = ErrorMessage{
			Error_message: error_message,
		}
		json.NewEncoder(res_wri).Encode(out_data)
	}
}

func unix_timestamp(res_wri http.ResponseWriter, requ *http.Request) {
	unix_ts := time.Now().Unix()
	out_data := map[string]int64{"unix_timestamp": unix_ts}
	json.NewEncoder(res_wri).Encode(out_data)
}

func check_argument(ok_values []string, arg_to_check string) bool {
	if slices.Contains(ok_values, arg_to_check) {
		return true
	}
	return false
}

func load_timezones() []string {
	tz_data, _ := os.ReadFile("timezones.dat")
	return strings.Split(string(tz_data), "\n")
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
	out_tz := url_vars.Get("outtz")
	out_location, out_tz_err := time.LoadLocation(out_tz)
	if out_tz_err == nil {
		// if no query argument than default behaviour is to send all data
		if !send_date && !send_time && !send_tz {
			send_date = true
			send_time = true
			send_tz = true
		}
		time_now := time.Now().In(out_location)
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
		json.NewEncoder(res_wri).Encode(out_data)
	} else {
		var out_data ErrorMessage
		err_message := wrong_timezone_message(out_tz)
		out_data.Error_message = err_message
		json.NewEncoder(res_wri).Encode(out_data)
	}
}

func list_timezones(res_wri http.ResponseWriter, requ *http.Request) {
	tz_list := load_timezones()
	json.NewEncoder(res_wri).Encode(tz_list)
}

type OutDatetimeData struct {
	Timezone       string
	DatetimeString string
}

type InDatetimeData struct {
	From_timezone  string
	To_timezone    string
	DatetimeString string
}

func convert_timezone(res_wri http.ResponseWriter, requ *http.Request) {
	var input_datetime InDatetimeData
	var output_datetime OutDatetimeData
	datetime_layout := "2006-01-02T15:04:05"
	dec_err := json.NewDecoder(requ.Body).Decode(&input_datetime)
	if dec_err != nil {
		http.Error(res_wri, dec_err.Error(), http.StatusBadRequest)
		return
	}
	from_location, f_loc_err := time.LoadLocation(input_datetime.From_timezone)
	if f_loc_err != nil {
		http.Error(res_wri, f_loc_err.Error(), http.StatusBadRequest)
		return
	}
	to_location, t_loc_err := time.LoadLocation(input_datetime.To_timezone)
	if t_loc_err != nil {
		http.Error(res_wri, t_loc_err.Error(), http.StatusBadRequest)
		return
	}
	date_time_to_convert, parse_err := time.ParseInLocation(
		datetime_layout,
		input_datetime.DatetimeString,
		from_location,
	)
	if parse_err != nil {
		fmt.Println(parse_err)
		http.Error(res_wri, parse_err.Error(), http.StatusBadRequest)
		return
	}
	converted_datetime := date_time_to_convert.In(to_location)
	output_datetime.DatetimeString = converted_datetime.Format(datetime_layout)
	output_datetime.Timezone = input_datetime.To_timezone
	json.NewEncoder(res_wri).Encode(output_datetime)

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
