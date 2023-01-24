package main

import (
	"evsys/core"
	"log"
)

func main() {

	centralSystem := core.NewCentralSystem()
	if err := centralSystem.Start(); err != nil {
		log.Println("start failed")
	}

}
