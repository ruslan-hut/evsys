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
	centralSystem.Start()

}
