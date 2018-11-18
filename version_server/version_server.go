package main

import (
	"net/http"
	"encoding/json"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"regexp"
)

type AppConfig struct {
	Host string `json:"host"`
	Port int 	`json:"port"`
	Path string `json:"path"`
}

var config AppConfig
var apps = make(map[string]string)
var path = "/check_version/"

func homepage(writer http.ResponseWriter, request *http.Request) {
	log.Println(request.URL.Path)
	page := "<div>welcome to atom version server</div>"
	writer.Write([]byte(page))
}

func handleCommonRequest(writer http.ResponseWriter, request *http.Request) {
	p := request.URL.Path
	log.Println(p)
	if p == "/" {
		homepage(writer, request)
		return
	} else if strings.Index(p, path) == 0 {
		handleAppVersion(writer, request)
		return
	}

	staticHandler := http.FileServer(http.Dir(config.Path))
	staticHandler.ServeHTTP(writer, request)
}

func handleAppVersion(writer http.ResponseWriter, request *http.Request) {
	log.Println("request " + request.URL.Path)
	appName := strings.Replace(request.URL.Path, path, "", -1)

	files, err := ioutil.ReadDir(config.Path)
	if err != nil {
		text := `{"code":-1, "description":"can not find app"}`
		log.Println("response", text)
		writer.Write([]byte(text))
		return
	}

	var t int64 = 0
	url := ""
	version := ""
	for _, f := range files {
		if strings.Index(f.Name(), appName) < 0 {
			continue
		}
		if t < f.ModTime().Unix() {
			t = f.ModTime().Unix()
			r := regexp.MustCompile(`[\d+\.]+\d+`)
			arr := r.FindAllString(f.Name(), -1)
			if len(arr) > 0 {
				version = arr[0]
			}
			url = "http://" + config.Host + ":" + strconv.Itoa(config.Port) + "/" + f.Name()
		}
	}

	content := 	map[string]string{"url": url, "version": version, "modified_time": strconv.FormatInt(t, 10)}
	sss, _ := json.Marshal(content);
	text := `{"code":0, "description":"", "content":` + string(sss) + `}`
	log.Println("response", text)
	writer.Write([]byte(text))
}

func loadAppConfig(s string) {
	text, err := ioutil.ReadFile(s)

	if err != nil {
		return
	}
	json.Unmarshal(text, &config)
}

func main() {
	apps = make(map[string]string)
	http.HandleFunc("/", handleCommonRequest)
	//config := loadAppConfig("./config.json")
	loadAppConfig("./config.json")

	log.Println("version server started")
	err := http.ListenAndServe(":" + strconv.Itoa(config.Port), nil)
	if err != nil {
		log.Fatalln("listen at " + strconv.Itoa(config.Port) + " failed")
	}
}

