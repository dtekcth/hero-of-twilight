package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
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
	Name        string `json:"name"`
	Description string `json:"description"`
	Link        string `json:"link"`
}

type Config struct {
	Token          string
	Url            string
	UpdateInterval int
	Services       []Service
}

var ServiceMutex sync.RWMutex
var Services []Service
var config   Config

//go:embed all:static
var staticFiles embed.FS

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

		// Add static services.
		services = append(services, config.Services...)

		ServiceMutex.Lock()
		Services = services
		ServiceMutex.Unlock()
	}
}

func handleApiV1Services(response http.ResponseWriter, request *http.Request) {
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

func middlewareLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		log.Printf("\"%s %s %s\" \"%s\"\n", request.Method, request.URL.Path, request.Proto, strings.Join(request.Header["User-Agent"], ", "))
		next.ServeHTTP(response, request)
	})
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	configBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal(err)
	}

	go update()

	// Setup and start server.
	mux := http.NewServeMux()
	serverRoot, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(serverRoot)))
	mux.HandleFunc("GET /api/v1/services", handleApiV1Services)

	if err := http.ListenAndServe(":8080", middlewareLogger(mux)); err != nil {
		log.Fatal(err)
	}
}
