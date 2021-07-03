package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/core/loadpoint"
	"github.com/andig/evcc/core/site"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/test"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Assets is the embedded assets file system
var Assets fs.FS

type chargeModeJSON struct {
	Mode api.ChargeMode `json:"mode"`
}

type targetSoCJSON struct {
	TargetSoC int `json:"targetSoC"`
}

type minSoCJSON struct {
	MinSoC int `json:"minSoC"`
}

type route struct {
	Methods     []string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// routeLogger traces matched routes including their executing time
func routeLogger(inner http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		inner.ServeHTTP(w, r)
		log.TRACE.Printf(
			"%s\t%s\t%s",
			r.Method,
			r.RequestURI,
			time.Since(start),
		)
	}
}

func indexHandler(site site.API) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")

		indexTemplate, err := fs.ReadFile(Assets, "index.html")
		if err != nil {
			log.FATAL.Print("httpd: failed to load embedded template:", err.Error())
			log.FATAL.Fatal("Make sure templates are included using the `release` build tag or use `make build`")
		}

		t, err := template.New("evcc").Delims("[[", "]]").Parse(string(indexTemplate))
		if err != nil {
			log.FATAL.Fatal("httpd: failed to create main page template:", err.Error())
		}

		if err := t.Execute(w, map[string]interface{}{
			"Version":    Version,
			"Commit":     Commit,
			"Configured": len(site.LoadPoints()),
		}); err != nil {
			log.ERROR.Println("httpd: failed to render main page:", err.Error())
		}
	})
}

// jsonHandler is a middleware that decorates responses with JSON and CORS headers
func jsonHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		h.ServeHTTP(w, r)
	})
}

func jsonResponse(w http.ResponseWriter, r *http.Request, content interface{}) {
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(content); err != nil {
		log.ERROR.Printf("httpd: failed to encode JSON: %v", err)
	}
}

// HealthHandler returns current charge mode
func HealthHandler(site site.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !site.Healthy() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}
}

// TemplatesHandler returns current charge mode
func TemplatesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		class, ok := vars["class"]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		type template = struct {
			Name   string `json:"name"`
			Sample string `json:"template"`
		}

		res := make([]template, 0)
		for _, conf := range test.ConfigTemplates(class) {
			typedSample := fmt.Sprintf("type: %s\n%s", conf.Type, conf.Sample)
			t := template{
				Name:   conf.Name,
				Sample: typedSample,
			}
			res = append(res, t)
		}

		jsonResponse(w, r, res)
	}
}

// StateHandler returns current charge mode
func StateHandler(cache *util.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cache.State()
		for _, k := range []string{"availableVersion", "releaseNotes"} {
			delete(res, k)
		}
		jsonResponse(w, r, res)
	}
}

// CurrentChargeModeHandler returns current charge mode
func CurrentChargeModeHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := chargeModeJSON{Mode: lp.GetMode()}
		jsonResponse(w, r, res)
	}
}

// ChargeModeHandler updates charge mode
func ChargeModeHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		modeS, ok := vars["mode"]
		mode := api.ChargeModeString(modeS)
		if mode == "" || string(mode) != modeS || !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		lp.SetMode(mode)

		res := chargeModeJSON{Mode: lp.GetMode()}
		jsonResponse(w, r, res)
	}
}

// CurrentTargetSoCHandler returns current target soc
func CurrentTargetSoCHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := targetSoCJSON{TargetSoC: lp.GetTargetSoC()}
		jsonResponse(w, r, res)
	}
}

// TargetSoCHandler updates target soc
func TargetSoCHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		socS, ok := vars["soc"]
		soc, err := strconv.ParseInt(socS, 10, 32)

		if ok && err == nil {
			err = lp.SetTargetSoC(int(soc))
		}

		if !ok || err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		res := targetSoCJSON{TargetSoC: lp.GetTargetSoC()}
		jsonResponse(w, r, res)
	}
}

// CurrentMinSoCHandler returns current minimum soc
func CurrentMinSoCHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := minSoCJSON{MinSoC: lp.GetMinSoC()}
		jsonResponse(w, r, res)
	}
}

// MinSoCHandler updates minimum soc
func MinSoCHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		socS, ok := vars["soc"]
		soc, err := strconv.ParseInt(socS, 10, 32)

		if ok && err == nil {
			err = lp.SetMinSoC(int(soc))
		}

		if !ok || err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		res := minSoCJSON{MinSoC: lp.GetMinSoC()}
		jsonResponse(w, r, res)
	}
}

// RemoteDemandHandler updates minimum soc
func RemoteDemandHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		demandS, ok := vars["demand"]

		var source string
		if ok {
			source, ok = vars["source"]
		}

		demand, err := loadpoint.RemoteDemandString(demandS)

		if !ok || err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		lp.RemoteControl(source, demand)

		res := struct {
			Demand loadpoint.RemoteDemand `json:"demand"`
			Source string                 `json:"source"`
		}{
			Source: source,
			Demand: demand,
		}

		jsonResponse(w, r, res)
	}
}

func timezone() *time.Location {
	tz := os.Getenv("TZ")
	if tz == "" {
		tz = "Local"
	}

	loc, _ := time.LoadLocation(tz)
	return loc
}

// TargetChargeHandler updates target soc
func TargetChargeHandler(loadpoint loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		socS, ok := vars["soc"]
		socV, err := strconv.ParseInt(socS, 10, 32)

		if !ok || err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		timeS, ok := vars["time"]
		timeV, err := time.ParseInLocation("2006-01-02T15:04:05", timeS, timezone())

		if !ok || err != nil {
			log.DEBUG.Printf("parse time: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		loadpoint.SetTargetCharge(timeV, int(socV))

		res := struct {
			SoC  int64     `json:"soc"`
			Time time.Time `json:"time"`
		}{
			SoC:  socV,
			Time: timeV,
		}

		jsonResponse(w, r, res)
	}
}

// SocketHandler attaches websocket handler to uri
func SocketHandler(hub *SocketHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ServeWebsocket(hub, w, r)
	}
}

// HTTPd wraps an http.Server and adds the root router
type HTTPd struct {
	*http.Server
}

// NewHTTPd creates HTTP server with configured routes for loadpoint
func NewHTTPd(url string, site site.API, hub *SocketHub, cache *util.Cache) *HTTPd {
	routes := map[string]route{
		"health":    {[]string{"GET"}, "/health", HealthHandler(site)},
		"state":     {[]string{"GET"}, "/state", StateHandler(cache)},
		"templates": {[]string{"GET"}, "/config/templates/{class:[a-z]+}", TemplatesHandler()},
	}

	router := mux.NewRouter().StrictSlash(true)

	// websocket
	router.HandleFunc("/ws", SocketHandler(hub))

	// static - individual handlers per root and folders
	static := router.PathPrefix("/").Subrouter()
	static.Use(handlers.CompressHandler)

	static.HandleFunc("/", indexHandler(site))
	for _, dir := range []string{"css", "js", "ico"} {
		static.PathPrefix("/" + dir).Handler(http.FileServer(http.FS(Assets)))
	}

	// api
	api := router.PathPrefix("/api").Subrouter()
	api.Use(jsonHandler)
	api.Use(handlers.CompressHandler)
	api.Use(handlers.CORS(
		handlers.AllowedHeaders([]string{
			"Accept", "Accept-Language", "Content-Language", "Content-Type", "Origin",
		}),
	))

	// site api
	for _, r := range routes {
		api.Methods(r.Methods...).Path(r.Pattern).Handler(r.HandlerFunc)
	}

	// loadpoint api
	for id, lp := range site.LoadPoints() {
		lpAPI := api.PathPrefix(fmt.Sprintf("/loadpoints/%d", id)).Subrouter()

		routes := map[string]route{
			"getmode":         {[]string{"GET"}, "/mode", CurrentChargeModeHandler(lp)},
			"setmode":         {[]string{"POST", "OPTIONS"}, "/mode/{mode:[a-z]+}", ChargeModeHandler(lp)},
			"gettargetsoc":    {[]string{"GET"}, "/targetsoc", CurrentTargetSoCHandler(lp)},
			"settargetsoc":    {[]string{"POST", "OPTIONS"}, "/targetsoc/{soc:[0-9]+}", TargetSoCHandler(lp)},
			"getminsoc":       {[]string{"GET"}, "/minsoc", CurrentMinSoCHandler(lp)},
			"setminsoc":       {[]string{"POST", "OPTIONS"}, "/minsoc/{soc:[0-9]+}", MinSoCHandler(lp)},
			"settargetcharge": {[]string{"POST", "OPTIONS"}, "/targetcharge/{soc:[0-9]+}/{time:[0-9TZ:-]+}", TargetChargeHandler(lp)},
			"remotedemand":    {[]string{"POST", "OPTIONS"}, "/remotedemand/{demand:[a-z]+}/{source}", RemoteDemandHandler(lp)},
		}

		for _, r := range routes {
			lpAPI.Methods(r.Methods...).Path(r.Pattern).Handler(r.HandlerFunc)
		}
	}

	srv := &HTTPd{
		Server: &http.Server{
			Addr:         url,
			Handler:      router,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
			ErrorLog:     log.ERROR,
		},
	}
	srv.SetKeepAlivesEnabled(true)

	return srv
}

// Router returns the main router
func (s *HTTPd) Router() *mux.Router {
	return s.Handler.(*mux.Router)
}
