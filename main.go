package main

import (
	"evsys/server"
	"log"
)

func main() {

	centralSystem, err := server.NewCentralSystem()
	if err != nil {
		log.Println("central system initialization failed", err)
		return
	}
	if err = centralSystem.Start(); err != nil {
		log.Println("start failed", err)
	}

}
