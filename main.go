package main

import (
	"evsys/server"
	"log"
	"time"
	_ "time/tzdata"
)

func main() {

	location, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		log.Println("time zone initialization failed", err)
		return
	}

	centralSystem, err := server.NewCentralSystem(location)
	if err != nil {
		log.Println("central system initialization failed", err)
		return
	}
	centralSystem.Start()

}
