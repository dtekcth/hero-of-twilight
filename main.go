package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
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
	UrlString      string `json:"url"`
	url            *url.URL
	UpdateInterval int
	Namespaces     []string
	Services       []Service
}

var serviceMutex sync.RWMutex
var services []Service
var config   Config
var errorLog *log.Logger

//go:embed all:static
var staticFiles embed.FS

func servicesFromTokenUrl(token string, baseUrl *url.URL, namespaces []string) (services []Service, err error) {
	// Build request URL.
	requestUrl := baseUrl.JoinPath("v1", "services")
	requestQuery := url.Values{}
	requestQuery.Set("namespace", "*")
	requestUrl.RawQuery = requestQuery.Encode()

	request, err := http.NewRequest("GET", requestUrl.String(), nil)
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
	var nomadNamespaces []NomadNamespace
	if response.StatusCode == http.StatusOK {
		err = json.NewDecoder(response.Body).Decode(&nomadNamespaces)
		if err != nil {
			return 
		}
	}

	// Extract services from pererred namespaces.
	nomadServices := make(map[string]NomadService)
	for _, preferredNamespace := range namespaces {
		index := slices.IndexFunc(nomadNamespaces, func (namespace NomadNamespace ) bool { return namespace.Name == preferredNamespace })
		if index == -1 {
			continue
		}

		for _, service := range nomadNamespaces[index].Services {
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
		newServices, err := servicesFromTokenUrl(config.Token, config.url, config.Namespaces)
		if err != nil {
			errorLog.Println(err)
			continue
		}

		// Add static services.
		newServices = append(newServices, config.Services...)

		// Update global list.
		serviceMutex.Lock()
		services = newServices
		serviceMutex.Unlock()
	}
}

func handleApiV1Services(response http.ResponseWriter, request *http.Request) {
	serviceMutex.RLock()
	encoded, err := json.Marshal(services)
	serviceMutex.RUnlock()

	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
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

func readConfig() {
	configBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		errorLog.Fatal(err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		errorLog.Fatal(err)
	}

	// Validate and set defaults
	config.url, err = url.Parse(config.UrlString)
	if err != nil {
		errorLog.Fatal(err)
	}

	if config.UpdateInterval == 0 {
		log.Println("\"updateInterval\" not specified in config.json, defaulting to 60s")
		config.UpdateInterval = 60
	}

	if len(config.Namespaces) == 0 {
		log.Println("\"namespaces\" not specified in config.json, defaulting to \"default\"")
		config.Namespaces = append(config.Namespaces, "default")
	}
}

func main() {
	// Setup loggers.
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	errorLog = log.New(os.Stderr, "", log.Flags())

	readConfig()

	go update()

	// Setup and start server.
	mux := http.NewServeMux()
	serverRoot, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(serverRoot)))
	mux.HandleFunc("GET /api/v1/services", handleApiV1Services)

	if err := http.ListenAndServe(":8080", middlewareLogger(mux)); err != nil {
		errorLog.Fatal(err)
	}
}
