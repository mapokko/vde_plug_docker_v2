package main

import (
	"fmt"

	"phocs/vde_plug_docker/vdenet"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/docker/go-plugins-helpers/network"
)

const unixSock = "/run/docker/plugins/vde.sock"
const dsFile = "/vde_plug_docker.json"
const dsDefaultDir = "/etc/docker"

var (
	dsPath    string
	debugMode = kingpin.Flag("debug", "Enable debug mode.").Bool()
	dsClean   = kingpin.Flag("clean", "Delete old the data store.").Bool()
	dsDir     = kingpin.Flag("dir-path", "Directory path of the data store.").String()
)

func main() {
	kingpin.Parse()
	fmt.Println(*debugMode)
	if *dsDir != "" {
		dsPath = *dsDir + dsFile
	} else {
		dsPath = dsDefaultDir + dsFile
	}
	if *debugMode {
		log.SetLevel(log.DebugLevel)
	}
	d := vdenet.NewDriver(dsPath, *dsClean)
	h := network.NewHandler(&d)
	if err := h.ServeUnix("vde", 0); err != nil {
		log.Fatal(err)
	}

}
