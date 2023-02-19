package datastore

import (
	"encoding/json"
	"io/ioutil"
	"sync"

	log "github.com/sirupsen/logrus"
)

// holds the path for the datastore and a mutex
type DataStore struct {
	sync.Mutex
	Path string
}

const OpenMode = 0644

// initial value for the datastore struct
var store = DataStore{
	Path: "./datastore.json",
}

// Sets a new path for the datastore file
func SetPath(path string) {
	store.Path = path
}

// Empties datastore file
func Clean() {
	// lock datastore mutex
	store.Lock()

	// unlock datasotre mutex when function ends
	defer store.Unlock()

	// empties out datastore file writing nil
	if err := ioutil.WriteFile(store.Path, nil, OpenMode); err != nil {
		log.Warnf("Datastore.Clean: [ %s ]", err)
	}
}

// Loads the networks in datastore file in the provided driver
// elem: Driver onto which the networks must be stored
func Load(elem interface{}) error {
	var err error

	// lock datastore mutex
	store.Lock()

	// unlock datastore mutex when function ends
	defer store.Unlock()

	// read datastore file
	if buf, err := ioutil.ReadFile(store.Path); err == nil {

		// since datastore file has JSON format, unmarshal it and stores it the Driver
		return json.Unmarshal(buf, &elem)
	} else {
		log.Warnf("Datastore.Load: [ %s ]", err)
	}
	return err
}

// Stores the network information from the Driver in the datastore
func Store(elem interface{}) error {
	var err error

	// datastore has JSON format, so the driver struct get marshaled
	if buf, err := json.Marshal(&elem); err == nil {

		// lock the datastore
		store.Lock()

		// write into datastore file
		err = ioutil.WriteFile(store.Path, buf, OpenMode)

		// unlock datastore
		store.Unlock()
	}
	if err != nil {
		log.Warnf("Datastore.Store: [ %s ]", err)
	}
	return err
}
