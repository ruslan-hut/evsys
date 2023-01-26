package main

import (
	"evsys/server"
	"log"
)

func main() {

	centralSystem := server.NewCentralSystem()
	if err := centralSystem.Start(); err != nil {
		log.Println("start failed")
	}

}
