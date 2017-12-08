package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	rpc "github.com/gorilla/rpc/v2"
	rpcjson "github.com/gorilla/rpc/v2/json2"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/openzipkin/zipkin-go-opentracing/examples/middleware"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/event"
)

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// setting up and running our http server
// rpc and replication live in rpc.go and replication.go respectively

func (state *InMemoryState) runServer() {
	go func() {
		// for debugging:
		// http://stackoverflow.com/questions/19094099/how-to-dump-goroutine-stacktraces
		log.Println(http.ListenAndServe(":6060", nil))
	}()
	r := rpc.NewServer()
	r.RegisterCodec(rpcjson.NewCodec(), "application/json")
	r.RegisterCodec(rpcjson.NewCodec(), "application/json;charset=UTF-8")
	d := NewDatameshRPC(state)
	err := r.RegisterService(d, "") // deduces name from type name
	if err != nil {
		log.Printf("Error while registering services %s", err)
	}

	tracer := opentracing.GlobalTracer()

	router := mux.NewRouter()
	router.Handle("/rpc",
		middleware.FromHTTPRequest(tracer, "rpc")(NewAuthHandler(r)),
	)

	allowRegistrations := os.Getenv("ALLOW_PUBLIC_REGISTRATION")
	if allowRegistrations != "" {
		router.Handle("/register",
			state.NewRegistrationServer(),
		)
	}

	router.HandleFunc("/status",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "OK")
		},
	)

	router.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui", 301)
		},
	)

	router.HandleFunc("/stripe",
		func(w http.ResponseWriter, r *http.Request) {
			stripe.Key = d.state.config.StripePrivateKey

			// read body from r, decode into e
			e := &stripe.Event{}

			requestBody, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", 400)
				return
			}

			log.Printf("[Stripe Handler] request body %+v", string(requestBody))

			err = json.Unmarshal(requestBody, e)
			if err != nil {
				http.Error(w, "Cannot unmarshal into event", 400)
				return
			}

			log.Printf("[Stripe Handler] before verify %+v", e)

			verified, err := event.Get(e.ID, nil)
			if err != nil {
				http.Error(w, "Nice try", 400)
				return
			}

			e = verified
			log.Printf("[Stripe Handler] after verify %+v", e)
			// Now safe to use e

			// TODO: do some stuff with the event, update user object to
			// appropriate tier (if we're being told that billing a renewal
			// just failed, set their tier to free).

		},
	)

	router.Handle(
		"/filesystems/{filesystem}/{fromSnap}/{toSnap}",
		middleware.FromHTTPRequest(tracer, "zfs-sender")(
			NewAuthHandler(state.NewZFSSendingServer()),
		),
	).Methods("GET")

	router.Handle(
		"/filesystems/{filesystem}/{fromSnap}/{toSnap}",
		middleware.FromHTTPRequest(tracer, "zfs-receiver")(
			NewAuthHandler(state.NewZFSReceivingServer()),
		),
	).Methods("POST")

	// setup a static file server from the configured directory
	// TODO: we need a way for /admin/some/sub/route to return frontendStaticFolder + '/admin/index.html'
	// this is to account for HTML5 routing which is the same index.html with lots of sub-routes the browser will sort out
	frontendStaticFolder := os.Getenv("FRONTEND_STATIC_FOLDER")
	if frontendStaticFolder == "" {
		frontendStaticFolder = "/www"
	}

	exists, err := pathExists(frontendStaticFolder)

	if exists {
		log.Printf(
			"Serving static frontend files from %s",
			frontendStaticFolder,
		)
		// trying to get the fonts to load in production
		injectFontHeaders := func(h http.Handler) http.HandlerFunc {
			var mimeTypes = map[string]string{
				".woff2": "font/woff2",
				".woff":  "application/x-font-woff",
				".ttf":   "application/font-sfnt",
				".eot":   "application/vnd.ms-fontobject",
			}
			return func(w http.ResponseWriter, r *http.Request) {
				ext := path.Ext(r.URL.Path)
				fmt.Println("ext")
				fmt.Println(ext)
				mimeType := mimeTypes[ext]
				fmt.Println("mimeType")
				fmt.Println(mimeType)
				if mimeType != "" {
					w.Header().Add("Content-Type", mimeType)
				}
				h.ServeHTTP(w, r)
			}
		}
		router.PathPrefix("/ui/").HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, frontendStaticFolder+"/ui/index.html")
			},
		)
		router.PathPrefix("/").Handler(
			http.StripPrefix("/",
				injectFontHeaders(http.FileServer(http.Dir(frontendStaticFolder))),
			),
		)
	}

	loggedRouter := handlers.LoggingHandler(getLogfile("requests"), router)
	err = http.ListenAndServe(":6969", loggedRouter)
	if err != nil {
		out(fmt.Sprintf("Unable to listen on port 6969: '%s'\n", err))
		log.Fatalf("Unable to listen on port 6969: '%s'", err)
	}
}

type AuthHandler struct {
	subHandler http.Handler
}

var DISABLE_BASIC_AUTH_NAME string = "disableBasicAuthWindow"

func auth(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	notAuth := func(w http.ResponseWriter) {
		disableBasicAuth := r.URL.Query().Get(DISABLE_BASIC_AUTH_NAME)
		if len(disableBasicAuth) <= 0 {
			w.Header().Set("WWW-Authenticate", "Basic")
		}
		http.Error(w, "Unauthorized.", 401)
	}
	// check for empty username, if so show a login box
	user, pass, _ := r.BasicAuth()
	if user == "" {
		notAuth(w)
		return r, fmt.Errorf("Permission denied.")
	}
	// ok, user has provided u/p, try to log them in
	authorized, passworded, err := CheckPassword(user, pass)
	if err != nil {
		log.Printf(
			"[AuthHandler] Error running check on %s: %s:",
			user, err,
		)
		http.Error(w, fmt.Sprintf("Error: %s.", err), 401)
		return r, err
	}
	if !authorized {
		notAuth(w)
		return r, fmt.Errorf("Permission denied.")
	}
	u, err := GetUserByName(user)
	if err != nil {
		log.Printf(
			"[AuthHandler] Unable to locate user %v: %v", user, err,
		)
		notAuth(w)
		return r, fmt.Errorf("Permission denied.")
	}
	r = r.WithContext(
		context.WithValue(context.WithValue(r.Context(), "authenticated-user-id", u.Id),
			"password-authenticated", passworded),
	)
	return r, nil
}

func (a AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r, err := auth(w, r)
	if err != nil {
		// Communicating the error upstream is handled by auth
		return
	}
	a.subHandler.ServeHTTP(w, r)
}

func NewAuthHandler(handler http.Handler) http.Handler {
	return AuthHandler{subHandler: handler}
}

func authHandlerFunc(f func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r, err := auth(w, r)
		if err != nil {
			return
		}
		f(w, r)
	}
}
