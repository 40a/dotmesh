package main

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"text/template"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	rpc "github.com/gorilla/rpc/v2"
	rpcjson "github.com/gorilla/rpc/v2/json2"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/openzipkin/zipkin-go-opentracing/examples/middleware"
)

// a crap web server

type WebServer struct {
	state *InMemoryState
}

func (s *InMemoryState) NewWebServer() http.Handler {
	return WebServer{
		state: s,
	}
}

func (web WebServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value("authenticated-user-id").(string)
	if userId == "" {
		// TODO all error cases should set error code in this fn
		fmt.Fprintf(w, "Error getting userId.")
		return
	}
	u, err := GetUserById(userId)
	if err != nil {
		log.Printf("[WebServer] %v", err)
		fmt.Fprintf(w, "Error getting user object.")
		return
	}
	// Sending the client the username and password in plaintext is clearly a
	// hack, and should be replaced by some proper crypto-based authn scheme.
	usernameJson, err := json.Marshal(u.Name)
	if err != nil {
		log.Printf("[WebServer] %v", err)
		fmt.Fprintf(w, "Error with username.")
		return
	}
	passwordJson, err := json.Marshal(u.ApiKey)
	if err != nil {
		log.Printf("[WebServer] %v", err)
		fmt.Fprintf(w, "Error with apiKey.")
		return
	}

	h := md5.New()
	io.WriteString(h, u.Email)
	emailHash := fmt.Sprintf("%x", h.Sum(nil))

	tmplStr := `
<!DOCTYPE html>
<html>

<head>
<meta charset='utf-8'>
<meta http-equiv="X-UA-Compatible" content="chrome=1">
<meta name="viewport" content="width=device-width, initial-scale=1.0">

<link rel="shortcut icon" type="image/png" href="{{.AssetsURLPrefix}}/images/datamesh.png">

<!-- CSS -->
<link href='https://fonts.googleapis.com/css?family=Roboto:500' rel='stylesheet' type='text/css'>
<link href='https://fonts.googleapis.com/css?family=Roboto+Condensed:300' rel='stylesheet' type='text/css'>
<link href='https://fonts.googleapis.com/css?family=Source+Sans+Pro:300' rel='stylesheet' type='text/css'>
<link rel="stylesheet" type="text/css" href="{{.AssetsURLPrefix}}/stylesheets/stylesheet.css" media="screen" />
<link rel="stylesheet" type="text/css" href="{{.AssetsURLPrefix}}/stylesheets/pygment_trac.css" media="screen" />
<link rel="stylesheet" type="text/css" href="{{.AssetsURLPrefix}}/stylesheets/print.css" media="print" />

<script src="{{.AssetsURLPrefix}}/scripts/clipboard.min.js"></script>
<script src="{{.AssetsURLPrefix}}/scripts/jquery.js"></script>
<script src="{{.AssetsURLPrefix}}/scripts/jquery.jsonrpc.js"></script>
<script src="{{.AssetsURLPrefix}}/scripts/jquery.dataTables.min.js"></script>
<script src="{{.AssetsURLPrefix}}/scripts/bootstrap.min.js"></script>
<link rel="stylesheet" href="{{.AssetsURLPrefix}}/stylesheets/jquery.dataTables.min.css" type="text/css" media="screen" />
<link rel="stylesheet" href="{{.AssetsURLPrefix}}/stylesheets/bootstrap.min.css" type="text/css" media="screen" />
<link rel="stylesheet" href="{{.AssetsURLPrefix}}/stylesheets/bootstrap-theme.min.css" type="text/css" media="screen" />
<script src="{{.AssetsURLPrefix}}/scripts/app.js"></script>

<title>Datamesh Console</title>

<script>
	var username = {{.UsernameJson}};
	var password = {{.PasswordJson}};
	var postLogoutURL = "{{.HomepageURL}}"; // TODO escape this.
	var emailHash = "{{.EmailHash}}";
</script>
</head>

<body>
  <div id="container">
    <header id="top">
      <div style="float:left;">
        <h1 style="margin:0;"><a href="{{.HomepageURL}}"><img src="{{.AssetsURLPrefix}}/images/datamesh-on-dark.png" class="icon" /> Datamesh Console</a></h1>
      </div>
      <div style="float:right;" id="top-navbar">
        <a href="{{.HomepageURL}}/docs/" class="button invisible"><span>Docs &amp; Install</span></a>
        <a href="https://github.com/lukemarsden/datamesh/" id="view-on-github" class="padded-button button invisible"><span>GitHub</span></a>
        <a href="http://eepurl.com/b7iEn1" class="button invisible" style="margin-left:10px;"><span>Newsletter</span></a>
		<a href="javascript:void(0);" onclick="alert('Email support@data-mesh.io to request modification to your account.')" class="button" style="margin-left:10px;"><span>Logged in as {{.UsernameHtml}}</span></a>
		<a href="javascript:void(0);" onclick="logout();" class="button cta" style="margin-left:10px;"><span>Log out</span></a>
      </div>
      <div style="clear:both;"></div>
    </header>

<div class="inner-body">
<div class="inner app">
<section id="main_content">
<div id="app">
Loading, please wait... (requires JavaScript)
</div>

<p>&nbsp;</p>

</section>
</div>
</div>
    <header id="top" style="height:auto;" class="actually-footer">
      <div style="float:right;">
          <h1 style="margin:0;"><a href="{{.HomepageURL}}"><img src="{{.AssetsURLPrefix}}/images/datamesh-on-dark.png" class="icon" /> Datamesh</a></h1>
      </div>
      <div style="margin:15px 0; color:#eee; float:left;">&copy; 2017 Luke Marsden&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</div>
      <div style="clear:both;"></div>
    </header>
  </div>
</body>

</html>
`
	type TemplateArgs struct {
		UsernameJson    string
		PasswordJson    string
		UsernameHtml    string
		AssetsURLPrefix string
		HomepageURL     string
		EmailHash       string
	}
	assetsURLPrefix := os.Getenv("ASSETS_URL_PREFIX")
	homepageURL := os.Getenv("HOMEPAGE_URL")
	t := TemplateArgs{
		UsernameJson:    string(usernameJson),
		PasswordJson:    string(passwordJson),
		EmailHash:       emailHash,
		UsernameHtml:    htmlEscape(u.Name),
		AssetsURLPrefix: assetsURLPrefix,
		HomepageURL:     homepageURL,
	}
	tmpl, err := template.New("t").Parse(tmplStr)
	if err != nil {
		log.Printf("[WebServer] %v", err)
		fmt.Fprintf(w, "Error with template.")
		return
	}
	err = tmpl.Execute(w, t)
	if err != nil {
		log.Printf("[WebServer] %v", err)
		fmt.Fprintf(w, "Error with template.")
		return
	}
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

	router.Handle("/ux", NewAuthHandler(state.NewWebServer()))

	router.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/register", 301)
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

func (a AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	notAuth := func(w http.ResponseWriter) {
		w.Header().Set("WWW-Authenticate", "Basic")
		http.Error(w, "Unauthorized.", 401)
	}
	// check for empty username, if so show a login box
	user, pass, _ := r.BasicAuth()
	if user == "" {
		notAuth(w)
		return
	}
	// ok, user has provided u/p, try to log them in
	authorized, err := check(user, pass)
	if err != nil {
		log.Printf(
			"[AuthHandler] Error running check on %s: %s:",
			user, err,
		)
		http.Error(w, fmt.Sprintf("Error: %s.", err), 401)
		return
	}
	if !authorized {
		notAuth(w)
		return
	}
	u, err := GetUserByName(user)
	if err != nil {
		log.Printf(
			"[AuthHandler] Unable to locate user %v: %v", user, err,
		)
		notAuth(w)
		return
	}
	r = r.WithContext(
		context.WithValue(r.Context(), "authenticated-user-id", u.Id),
	)
	a.subHandler.ServeHTTP(w, r)
}

func NewAuthHandler(handler http.Handler) http.Handler {
	return AuthHandler{subHandler: handler}
}

func getPassword(user string) (string, error) {
	users, err := AllUsers()
	if err != nil {
		return "", err
	}
	for _, u := range users {
		if u.Name == user {
			return u.ApiKey, nil
		}
	}
	return "", fmt.Errorf("Unable to find user %v", user)
}

func check(u, p string) (bool, error) {
	password, err := getPassword(u)
	if err != nil {
		return false, err
	} else {
		// TODO think more about timing attacks
		return (subtle.ConstantTimeCompare(
			[]byte(password),
			[]byte(p)) == 1), nil
	}
}
