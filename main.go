package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	"io/ioutil"
)

type NomadService struct {
	Name string `json:"ServiceName"`
	Tags []string
}

type NomadNamespace struct {
	Name     string `json:"Namespace"`
	Services []NomadService
}

type Service struct {
	Name        string
	Description string
	Link        string
}

type Config struct {
	Token          string
	Url            string
	UpdateInterval int
	Services       []Service
}

var Templates *template.Template
var ServiceMutex sync.RWMutex
var Services []Service
var config   Config

func servicesFromTokenUrl(token, url string) (services []Service, err error) {
	request, err := http.NewRequest("GET", url + "/v1/services?namespace=*", nil)
	if err != nil {
		return
	}
	request.Header.Add("X-Nomad-Token", token)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()

	// Get namespaces from request.
	var namespaces []NomadNamespace
	if response.StatusCode == http.StatusOK {
		err = json.NewDecoder(response.Body).Decode(&namespaces)
		if err != nil {
			return 
		}
	}

	// Extract services from pererred namespaces.
	nomadServices := make(map[string]NomadService)
	preferredNamespaces := []string{ "prod", "backup" }
	for _, preferredNamespace := range preferredNamespaces {
		index := slices.IndexFunc(namespaces, func (namespace NomadNamespace ) bool { return namespace.Name == preferredNamespace })
		if index == -1 {
			continue
		}

		for _, service := range namespaces[index].Services {
			// Skip if have already registered this service.
			if _, ok := nomadServices[service.Name]; ok {
				continue
			}

			nomadServices[service.Name] = service
		}
	}

	// Extract link discovery information from services.
	for _, nomadService := range nomadServices {
		// Extract tags to key-value map.
		tags := make(map[string]string)
		for _, tag := range nomadService.Tags {
			key, value, _ := strings.Cut(tag, "=")
			tags[key] = value
		}

		// Extract required tags.
		name,        hasName        := tags["link-discovery.name"]
		description, hasDescription := tags["link-discovery.description"]
		link,        hasLink        := tags["link-discovery.link"]

		if !(hasName && hasDescription && hasLink) {
			continue
		}

		service := Service {
			Name:        name,
			Description: description,
			Link:        link,
		}

		services = append(services, service)
	}

	return
}

func update() {
	updateInterval := time.Tick(time.Duration(config.UpdateInterval) * time.Second)
	for ;; <-updateInterval {
		services, err := servicesFromTokenUrl(config.Token, config.Url)
		if err != nil {
			log.Println(err)
			return
		}

		// Add static services and sort on name.
		services = append(services, config.Services...)
		slices.SortFunc(services, func(a, b Service) int { return strings.Compare(a.Name, b.Name) })

		ServiceMutex.Lock()
		Services = services
		ServiceMutex.Unlock()
	}
}

func handleApiV1Services(response http.ResponseWriter, request *http.Request) {
	if request.Method != "GET" {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ServiceMutex.RLock()
	encoded, err := json.Marshal(Services)
	ServiceMutex.RUnlock()

	if err != nil {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(encoded)
}

func handleIndex(response http.ResponseWriter, request *http.Request) {
	if request.Method != "GET" {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ServiceMutex.RLock()
	err := Templates.ExecuteTemplate(response, "index.html", Services)
	ServiceMutex.RUnlock()
	if err != nil {
		log.Println(err)
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	configBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal(err)
	}

	// Load templates.
	Templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	go update()

	// Setup and start server.
	http.HandleFunc("/", handleIndex)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/api/v1/services", handleApiV1Services)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
