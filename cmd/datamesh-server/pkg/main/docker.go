package main

// docker volume plugin for providing datamesh volumes to docker via e.g.
// docker run -v name:/path --volume-driver=dm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)

const PLUGINS_DIR = "/run/docker/plugins"
const DM_SOCKET = PLUGINS_DIR + "/dm.sock"

type ResponseImplements struct {
	// A response to the Plugin.Activate request
	Implements []string
}

type RequestCreate struct {
	// A request to create a volume for Docker
	Name string
	Opts map[string]string
}

type RequestMount struct {
	// A request to mount a volume for Docker
	Name string
}

type RequestRemove struct {
	// A request to remove a volume for Docker
	Name string
}

type ResponseSimple struct {
	// A response which only indicates if there was an error or not
	Err string
}

type ResponseMount struct {
	// A response to the VolumeDriver.Mount request
	Mountpoint string
	Err        string
}

type ResponseListVolume struct {
	// Used in the JSON representation of ResponseList
	Name       string
	Mountpoint string
}

type ResponseList struct {
	// A response which enumerates volumes for VolumeDriver.List
	Volumes []ResponseListVolume
	Err     string
}

// create a symlink from /datamesh/:name[@:branch] into /dmfs/:filesystemId
func newContainerMountSymlink(name VolumeName, filesystemId string) (string, error) {
	if _, err := os.Stat(CONTAINER_MOUNT_PREFIX); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(CONTAINER_MOUNT_PREFIX, 0700); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	if _, err := os.Stat(containerMntParent(name)); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(containerMntParent(name), 0700); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	result := containerMnt(name)
	// Only create it if it doesn't already exist. Otherwise just hand it back
	// (the target of it may have been updated elsewhere).
	if _, err := os.Stat(result); err != nil {
		if os.IsNotExist(err) {
			err = os.Symlink(mnt(filesystemId), result)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	return result, nil
}

func (state *InMemoryState) mustCleanupSocket() {
	if _, err := os.Stat(PLUGINS_DIR); err != nil {
		if err := os.MkdirAll(PLUGINS_DIR, 0700); err != nil {
			log.Fatalf("Could not make plugin directory %s: %v", PLUGINS_DIR, err)
		}
	}
	if _, err := os.Stat(DM_SOCKET); err == nil {
		if err = os.Remove(DM_SOCKET); err != nil {
			log.Fatalf("Could not clean up existing socket at %s: %v", DM_SOCKET, err)
		}
	}
}

// Annotate a context with admin-level authorization.
func AdminContext(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, "authenticated-user-id", ADMIN_USER_UUID)
	return ctx
}

func (state *InMemoryState) runPlugin() {
	log.Print("Starting dm plugin")

	// docker acts like the admin user, for now.
	ctx := AdminContext(context.Background())

	state.mustCleanupSocket()

	http.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		log.Print("<= /Plugin.Activate")
		responseJSON, _ := json.Marshal(&ResponseImplements{
			Implements: []string{"VolumeDriver"},
		})
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
	})
	http.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		log.Print("<= /VolumeDriver.Create")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		request := new(RequestCreate)
		err = json.Unmarshal(requestJSON, request)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		// for now, just name the volumes as requested by the user. later,
		// adding ids and per-fs metadata may be useful.

		if _, err := state.procureFilesystem(ctx, name); err != nil {
			writeResponseErr(err, w)
			return
		}
		// TODO acquire containerRuntimeLock and update our state and etcd with
		// the fact that a container will soon be running on this volume...
		writeResponseOK(w)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	http.HandleFunc("/VolumeDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		/*
			We do not actually want to remove the dm volume when Docker
			references to them are removed.

			This is a no-op.
		*/
		writeResponseOK(w)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	http.HandleFunc("/VolumeDriver.Path", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Only return the path if it's actually active on the local host.
		log.Print("<= /VolumeDriver.Path")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		request := new(RequestMount)
		if err := json.Unmarshal(requestJSON, request); err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		log.Printf("Mountpoint for %s: %s", name, containerMnt(name))
		responseJSON, _ := json.Marshal(&ResponseMount{
			Mountpoint: containerMnt(name),
			Err:        "",
		})
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	http.HandleFunc("/VolumeDriver.Mount", func(w http.ResponseWriter, r *http.Request) {
		// TODO acquire containerRuntimeLock and update our state and etcd with
		// the fact that a container will soon be running on this volume...
		log.Print("<= /VolumeDriver.Mount")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatalf("Unable to read response body %s", err)
		}
		request := new(RequestMount)
		if err := json.Unmarshal(requestJSON, request); err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		filesystemId, err := state.procureFilesystem(ctx, name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		mountpoint, err := newContainerMountSymlink(name, filesystemId)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		// Allow things that don't want containers to start during their
		// operations to delay the start of a container. Commented out because
		// it causes a deadlock.
		/*
			state.containersLock.Lock()
			defer state.containersLock.Unlock()
		*/

		log.Printf("Mountpoint for %s: %s", name, mountpoint)
		responseJSON, _ := json.Marshal(&ResponseMount{
			Mountpoint: mountpoint,
			Err:        "",
		})
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)

		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	http.HandleFunc("/VolumeDriver.Unmount", func(w http.ResponseWriter, r *http.Request) {
		// TODO acquire containerRuntimeLock and update our state and etcd with
		// the fact that one less container is now running on this volume...
		writeResponseOK(w)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	http.HandleFunc("/VolumeDriver.List", func(w http.ResponseWriter, r *http.Request) {
		log.Print("<= /VolumeDriver.List")
		var response = ResponseList{
			Err: "",
		}

		for _, fs := range (*state).registry.Filesystems() {
			log.Printf("Mountpoint for %s: %s", fs, containerMnt(fs))
			response.Volumes = append(response.Volumes, ResponseListVolume{
				Name:       fs.String(),
				Mountpoint: containerMnt(fs),
			})
		}

		responseJSON, _ := json.Marshal(response)
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
		// asynchronously notify datamesh that the containers running on a
		// volume may have changed
		go func() { state.fetchRelatedContainersChan <- true }()
	})

	listener, err := net.Listen("unix", DM_SOCKET)
	if err != nil {
		log.Fatalf("Could not listen on %s: %v", DM_SOCKET, err)
	}

	http.Serve(listener, nil)
}

func (state *InMemoryState) runErrorPlugin() {
	// A variant of the normal plugin which just returns immediately with
	// errors. For bootstrapping.
	log.Print("Starting dm temporary bootstrap plugin")
	state.mustCleanupSocket()
	http.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /Plugin.Activate")
		responseJSON, _ := json.Marshal(&ResponseImplements{
			Implements: []string{"VolumeDriver"},
		})
		w.Write(responseJSON)
	})
	http.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Create")
		writeResponseErr(fmt.Errorf("I'm sorry Dave, I can't do that. I'm still starting up."), w)
	})
	http.HandleFunc("/VolumeDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Remove")
		writeResponseOK(w)
	})
	http.HandleFunc("/VolumeDriver.Path", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Path")
		requestJSON, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeResponseErr(err, w)
			return
		}
		request := new(RequestMount)
		if err := json.Unmarshal(requestJSON, request); err != nil {
			writeResponseErr(err, w)
			return
		}
		namespace, localName, err := parseNamespacedVolume(request.Name)
		if err != nil {
			writeResponseErr(err, w)
			return
		}

		name := VolumeName{namespace, localName}

		log.Printf("Mountpoint for %s: %s", name, containerMnt(name))
		responseJSON, _ := json.Marshal(&ResponseMount{
			Mountpoint: containerMnt(name),
			Err:        "",
		})
		log.Printf("=> %s", string(responseJSON))
		w.Write(responseJSON)
	})
	http.HandleFunc("/VolumeDriver.Mount", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Mount")
		writeResponseErr(fmt.Errorf("datamesh still starting or datamesh-etcd unable to achieve quorum"), w)
	})
	http.HandleFunc("/VolumeDriver.Unmount", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.Unmount")
		writeResponseErr(fmt.Errorf("datamesh still starting or datamesh-etcd unable to achieve quorum"), w)
	})
	http.HandleFunc("/VolumeDriver.List", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[bootstrap] /VolumeDriver.List")
		var response = ResponseList{
			Err: "datamesh still starting or datamesh-etcd unable to achieve quorum",
		}
		responseJSON, _ := json.Marshal(response)
		w.Write(responseJSON)
	})
	listener, err := net.Listen("unix", DM_SOCKET)
	if err != nil {
		log.Fatalf("Could not listen on %s: %v", DM_SOCKET, err)
	}
	http.Serve(listener, nil)
}

func writeResponseOK(w http.ResponseWriter) {
	// A shortcut to writing a ResponseOK to w
	responseJSON, _ := json.Marshal(&ResponseSimple{Err: ""})
	w.Write(responseJSON)
}

func writeResponseErr(err error, w http.ResponseWriter) {
	// A shortcut to responding with an error, and then log the error
	errString := fmt.Sprintln(err)
	log.Printf("Error: %v", err)
	responseJSON, _ := json.Marshal(&ResponseSimple{Err: errString})
	w.Write(responseJSON)
}
