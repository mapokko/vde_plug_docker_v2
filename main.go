package main

import (

	// network interface functions
	"phocs/vde_plug_docker/vdenet"

	// logging library
	log "github.com/sirupsen/logrus"

	// bash flags parser library
	"gopkg.in/alecthomas/kingpin.v2"

	// docker plugin helper functions
	"github.com/docker/go-plugins-helpers/network"
)

/*
default values in case of missing bash parameters

unixSock: defalut position for UNIX socket to enable IPC with docker engine
dsFile: datastore filename
dsDefaultDir: default position for datastore file
*/
const unixSock = "/run/docker/plugins/vde.sock"
const dsFile = "/vde_plug_docker.json"
const dsDefaultDir = "/etc/docker"

var (
	dsPath string
	// flags for this docker plugin
	debugMode = kingpin.Flag("debug", "Enable debug mode.").Bool()
	dsClean   = kingpin.Flag("clean", "Delete old the data store.").Bool()
	dsDir     = kingpin.Flag("dir-path", "Directory path of the data store.").String()
)

func main() {
	// get flags
	kingpin.Parse()

	//check if datastore path have been provided
	if *dsDir != "" {
		dsPath = *dsDir + dsFile
	} else {
		dsPath = dsDefaultDir + dsFile
	}

	// check if debug mode flag is true
	if *debugMode {
		log.SetLevel(log.DebugLevel)
	}

	// get network driver
	d := vdenet.NewDriver(dsPath, *dsClean)

	// provide the docker NetworkController with the network driver
	h := network.NewHandler(&d)

	// creates Unix socket if doesn't exists and starts listening for requests, socket name is first parameter
	if err := h.ServeUnix("vde", 0); err != nil {
		log.Fatal(err)
	}

}
