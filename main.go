package main

import (
	"evsys/internal/client"
	"evsys/internal/config"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
)

//const (
//	listenTypeSocket = "sock"
//	socketFile       = "ev.sock"
//)

func main() {
	cfg := config.GetConfig()
	router := httprouter.New()

	clientHandler := client.NewHandler()
	clientHandler.Register(router)

	start(cfg, router)
}

func start(cfg *config.Config, router *httprouter.Router) {

	//var listener net.Listener
	//var listenerErr error
	serverAddress := fmt.Sprintf("%s:%s", cfg.Listen.BindIP, cfg.Listen.Port)

	//if cfg.Listen.Type == listenTypeSocket {
	//	log.Println("initialise socket connection")
	//	appPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	socketPath := path.Join(appPath, socketFile)
	//	log.Printf("create listener on socket: %s", socketPath)
	//	listener, listenerErr = net.Listen("unix", socketPath)
	//} else {
	//	serverAddress := fmt.Sprintf("%s:%s", cfg.Listen.BindIP, cfg.Listen.Port)
	//	log.Printf("create listener on port: %s", serverAddress)
	//	listener, listenerErr = net.Listen("tcp", serverAddress)
	//}
	//
	//if listenerErr != nil {
	//	log.Fatal(listenerErr)
	//}

	//server := &http.Server{
	//	Handler:      router,
	//	WriteTimeout: 15 * time.Second,
	//	ReadTimeout:  15 * time.Second,
	//}

	log.Printf("starting server on %s", serverAddress)
	log.Fatal(http.ListenAndServe(serverAddress, router))

}
