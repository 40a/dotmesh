package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"text/template"

	"github.com/coreos/etcd/client"
)

// registration server, so that, if enabled with ALLOW_PUBLIC_REGISTRATION,
// humans can sign up for an account on this datamesh cluster

type RegistrationServer struct {
	state *InMemoryState
}

func (s *InMemoryState) NewRegistrationServer() http.Handler {
	return RegistrationServer{
		state: s,
	}
}

func HasContentType(r *http.Request, mimetype string) bool {
	contentType := r.Header.Get("Content-type")
	if contentType == "" {
		return mimetype == "application/octet-stream"
	}

	for _, v := range strings.Split(contentType, ",") {
		t, _, err := mime.ParseMediaType(v)
		if err != nil {
			break
		}
		if t == mimetype {
			return true
		}
	}
	return false
}

func IsRequestJSON(r *http.Request) bool {
	return HasContentType(r, "application/json")
}

func WriteError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(fmt.Sprintf("Internal server error. Please check logs.")))
}

type RegistrationPayload struct {
	Name          string
	Email         string
	Password      string
	EmailError    string
	NameError     string
	PasswordError string
	Created       bool
	Submit        bool
	Json          bool
}

type JSONPayload struct {
	Name     string `json:"Name"`
	Email    string `json:"Email"`
	Password string `json:"Password"`
}

func (payload *RegistrationPayload) Validate() bool {
	if payload.Password == "" {
		payload.PasswordError = "Password cannot be empty."
	}
	if payload.Email == "" {
		payload.EmailError = "Email address cannot be empty."
	}
	if payload.Name == "" {
		payload.NameError = "Name cannot be empty."
	} else if strings.Contains(payload.Name, "/") {
		payload.NameError = "Invalid username."
	}
	return payload.EmailError == "" && payload.NameError == "" && payload.PasswordError == ""
}

func NewRegistrationPayload(r *http.Request) (RegistrationPayload, error) {
	payload := RegistrationPayload{
		Name:          "",
		Email:         "",
		Password:      "",
		EmailError:    "",
		NameError:     "",
		PasswordError: "",
		Created:       false,
		Submit:        false,
		Json:          false,
	}

	if IsRequestJSON(r) {
		body, err := ioutil.ReadAll(r.Body)

		if err != nil {
			log.Printf("[RegistrationServer] Error reading HTTP body: %v", err)
			return payload, err
		}

		log.Println(string(body))
		var jsonPacket JSONPayload
		err = json.Unmarshal(body, &jsonPacket)

		if err != nil {
			log.Printf("[RegistrationServer] Error decoding JSON payload: %v - %s", err, body)
			return payload, err
		}

		payload.Name = jsonPacket.Name
		payload.Email = jsonPacket.Email
		payload.Password = jsonPacket.Password
		payload.Json = true
		payload.Submit = true

	} else {
		r.ParseForm()
		payload.Name = r.FormValue("username")
		payload.Email = r.FormValue("email")
		payload.Password = r.FormValue("password")
		payload.Submit = r.FormValue("submit") != ""
	}

	log.Printf("[RegistrationServer] payload: %v", payload)

	return payload, nil
}

func (web *RegistrationServer) registerUser(payload *RegistrationPayload) error {
	log.Printf("[RegistrationServer] registerUser: %v", payload)
	kapi, err := getEtcdKeysApi()
	if err != nil {
		log.Printf("[RegistrationServer] Error talking to etcd: %v", err)
		return err
	}

	if payload.Validate() {
		// lookup username in etcd, bail if it exists
		_, err = kapi.Get(
			context.Background(),
			fmt.Sprintf(
				"%s/users/%s", ETCD_PREFIX, payload.Name,
			),
			nil,
		)
		if !client.IsKeyNotFound(err) && err != nil {
			log.Printf("[RegistrationServer] Error checking username %v: %v", payload.Name, err)
			return err
		}
		if err == nil {
			payload.NameError = "Name already exists, please choose another."
		}
	}

	// validate the second time because we have just loaded the UsernameError
	if payload.Validate() {
		user, err := NewUser(payload.Name, payload.Email, payload.Password)
		if err != nil {
			log.Printf("[RegistrationServer] Error creating user %v: %v", payload.Name, err)
			return err
		}
		err = user.Save()
		if err != nil {
			log.Printf("[RegistrationServer] Error saving user %v: %v", payload.Name, err)
			return err
		}
		payload.Created = true
	}

	return nil
}

func (web RegistrationServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	payload, err := NewRegistrationPayload(r)

	if err != nil {
		WriteError(w)
		return
	}

	if payload.Submit {
		err := web.registerUser(&payload)
		if err != nil {
			WriteError(w)
			return
		}
	}

	if payload.Json {
		web.RespondJSON(&payload, w, r)
	} else {
		web.RespondHTML(&payload, w, r)
	}
}

func (web RegistrationServer) RespondHTML(payload *RegistrationPayload, w http.ResponseWriter, r *http.Request) {
	tmplStr := `<!DOCTYPE html>
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
<link rel="stylesheet" href="{{.AssetsURLPrefix}}/stylesheets/jquery.dataTables.min.css" type="text/css" media="screen" />

<title>Register for the ultimate data platform for containerized apps</title>

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
        <a href="/ux" class="button" style="margin-left:10px;"><span>Sign in</span></a>
        <a href="/register" class="button cta" style="margin-left:10px;"><span>Sign up</span></a>
      </div>
      <div style="clear:both;"></div>
    </header>

<div class="inner-header">
<div class="inner">
  <section id="main_content">
  <header class="post-header">
  <h1>Register for the ultimate data platform for containerized apps</h1>
  </header>
  </section>
</div>
</div>
<div class="inner-body">
<div class="inner">
<section id="main_content">
<div class="box-wide">

	{{if .Complete}}
		<h1>Account created, thank you!</h1>
		<p>Now you can <a href="/ux">log in</a>.</p>
	{{else}}
		<form action="/register" method="POST" class="register">
			<p>
				<div class="label"><p>Your Email Address</p></div>
				<input type="email" name="email" value="{{.FormEmail}}" />
				<span class="error">{{.ErrorEmail}}</span>
			</p>
			<p>
				<div class="label"><p>Choose Username</p></div>
				<input type="username" name="username" value="{{.FormUsername}}" />
				<span class="error">{{.ErrorUsername}}</span>
			</p>
			<p>
				<div class="label"><p>Choose Password<br />(also used as your API key)</p></div>
				<input type="password" name="password" value="{{.FormPassword}}" />
				<span class="error">{{.ErrorPassword}}</span>
			</p>
			<p style="clear:both; text-align:center;">
				<input type="submit" name="submit" class="button cta" value="Register" />
			</p>
		</form>
	{{end}}
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
		FormEmail       string
		ErrorEmail      string
		FormUsername    string
		ErrorUsername   string
		FormPassword    string
		ErrorPassword   string
		AssetsURLPrefix string
		HomepageURL     string
		Complete        bool
	}
	assetsURLPrefix := os.Getenv("ASSETS_URL_PREFIX")
	homepageURL := os.Getenv("HOMEPAGE_URL")
	t := TemplateArgs{
		FormEmail:       htmlEscape(payload.Email),
		ErrorEmail:      payload.EmailError,
		FormUsername:    htmlEscape(payload.Name),
		ErrorUsername:   payload.NameError,
		FormPassword:    htmlEscape(payload.Password),
		ErrorPassword:   payload.PasswordError,
		AssetsURLPrefix: assetsURLPrefix,
		HomepageURL:     homepageURL,
		Complete:        payload.Created,
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

func (web RegistrationServer) RespondJSON(payload *RegistrationPayload, w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(payload)
}

// copied from the stdlib

var htmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	// "&#34;" is shorter than "&quot;".
	`"`, "&#34;",
	// "&#39;" is shorter than "&apos;" and apos was not in HTML until HTML5.
	"'", "&#39;",
)

func htmlEscape(s string) string {
	return htmlReplacer.Replace(s)
}
