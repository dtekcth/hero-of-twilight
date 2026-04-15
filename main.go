package main

import (
	"embed"
	"encoding/json"
	"flag"
	"html/template"
	"io"
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
	Category    string `json:"category"`
}

type Category struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Config struct {
	Name           string
	Description    string
	Token          string
	UrlString      string `json:"url"`
	url            *url.URL
	UpdateInterval int
	Namespaces     []string
	Services       []Service
	Categories     []Category
}

type TemplateExecutor interface {
	ExecuteTemplate(writer io.Writer, name string, data any) error
}

type DebugTemplateExecutor struct {
	Glob string
}

func (executor DebugTemplateExecutor) ExecuteTemplate(writer io.Writer, name string, data any) error {
	templates, err := template.ParseGlob(executor.Glob)
	if err != nil {
		return err
	}
	return templates.ExecuteTemplate(writer, name,  data)
}

var serviceMutex sync.RWMutex
var services []Service
var config   Config
var errorLog *log.Logger
var templateExecutor TemplateExecutor

//go:embed all:static
var staticFiles embed.FS
//go:embed all:templates
var templateFiles embed.FS

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
			Category:    tags["link-discovery.category"],
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

		// Apply default values to optional fields.
		for i, service := range newServices {
			if service.Category == "" {
				service.Category = "default"
			}

			newServices[i] = service
		}

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

func handleApiV1Categories(response http.ResponseWriter, request *http.Request) {
	serviceMutex.RLock()
	encoded, err := json.Marshal(config.Categories)
	serviceMutex.RUnlock()

	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(encoded)
}

func handleIndex(response http.ResponseWriter, request *http.Request) {
	err := templateExecutor.ExecuteTemplate(response, "index.gohtml", config)
	if err != nil {
		errorLog.Println(err)
	}
}

func middlewareLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		log.Printf("\"%s %s %s\" \"%s\"\n", request.Method, request.URL.Path, request.Proto, strings.Join(request.Header["User-Agent"], ", "))
		next.ServeHTTP(response, request)
	})
}

func readConfig() {
	configBytes, err := os.ReadFile("config.json")
	if err != nil {
		errorLog.Fatal(err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		errorLog.Fatal(err)
	}

	// Validate and set defaults.
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

	categoryIndex := 0
	for i, category := range config.Categories {
		isBad := false
		if category.Id == "" {
			isBad = true
			log.Printf("Category at index %v has an empty id, removing\n", i)
		}

		if category.Name == "" {
			isBad = true
			log.Printf("Category at index %v has an empty name, removing\n", i)
		}

		if !isBad {
			config.Categories[categoryIndex] = category
			categoryIndex += 1
		}
	}
	config.Categories = config.Categories[:categoryIndex]

	if len(config.Categories) == 0 {
		log.Println("\"categories\" not specified in config.json, defaulting to \"default: Default\"")
		config.Categories = append(config.Categories, Category {Id: "default", Name: "Default"})
	}
}

func main() {
	// Setup loggers.
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	errorLog = log.New(os.Stderr, "", log.Flags())

	// Parse command line arguments.
	reload := flag.Bool("reload", false, "reload static files and templates on page refresh")
	flag.Parse()

	readConfig()

	// Setup handlers for reloading of static files.
	var staticHandler http.Handler
	if *reload {
		templateExecutor = DebugTemplateExecutor{"templates/*.gohtml"}
		staticHandler    = http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	} else {
		templateExecutor = template.Must(template.ParseFS(templateFiles, "**/*.gohtml"))
		staticHandler    = http.FileServerFS(staticFiles)
	}

	go update()

	// Setup and start server.
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.Handle("/static/", staticHandler)
	mux.HandleFunc("GET /api/v1/services",   handleApiV1Services)
	mux.HandleFunc("GET /api/v1/categories", handleApiV1Categories)

	if err := http.ListenAndServe(":8080", middlewareLogger(mux)); err != nil {
		errorLog.Fatal(err)
	}
}
