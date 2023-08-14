package main

import (
	"evsys/internal/config"
	"evsys/server"
	"flag"
	"fmt"
	"log"
	_ "time/tzdata"
)

func main() {

	configPath := flag.String("conf", "config.yml", "path to config file")
	flag.Parse()
	log.Println("using config file: " + *configPath)

	conf, err := config.GetConfig(configPath)
	if err != nil {
		log.Println(fmt.Sprintf("loading configuration failed: %s", err))
		return
	}
	if conf.IsDebug {
		log.Println("debug mode is enabled")
	}

	centralSystem, err := server.NewCentralSystem(conf)
	if err != nil {
		log.Println("central system initialization failed", err)
		return
	}
	centralSystem.Start()

}
