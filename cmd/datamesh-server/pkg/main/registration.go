package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"strings"
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
	Valid         bool
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
	payload.Valid = (payload.EmailError == "") && (payload.NameError == "") && (payload.PasswordError == "")
	return payload.Valid
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
		Valid:         false,
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

	// validate the second time because we have just loaded the UsernameError
	if payload.Validate() {
		user, err := NewUser(payload.Name, payload.Email, payload.Password)
		success := true
		if err != nil {
			log.Printf("[RegistrationServer] Error creating user %v: %v", payload.Name, err)
			success = false
			payload.NameError = fmt.Sprintf("Error saving user: %v", err)
		} else {
			err = user.Save()
			if err != nil {
				log.Printf("[RegistrationServer] Error saving user %v: %v", payload.Name, err)
				success = false
				payload.NameError = fmt.Sprintf("Error saving user: %v", err)
			}
		}
		payload.Created = success
	}

	return nil
}

func (web RegistrationServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// capture old 301 redirects to here and show the new UI
	if r.Method == "GET" {
		http.Redirect(w, r, "/ui", 301)
		return
	}

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
	} else {
		http.Error(w, "JSON request expected", 400)
		return
	}

	if !payload.Valid {
		w.WriteHeader(http.StatusBadRequest)
	}

	if payload.Json {
		web.RespondJSON(&payload, w, r)
	} else {
		http.Error(w, "JSON request expected", 400)
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
