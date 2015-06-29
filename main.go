package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/context"
	"github.com/justinas/alice"
	"gopkg.in/mgo.v2"
)

func main() {
	log.SetPrefix("[main] ")

	if os.Getenv(EnvDialURL) == "" {
		log.Fatalln("Environment not configured. Exiting...")
	}

	log.Printf("Connecting to %v", os.Getenv(EnvDialURL))
	session, err := mgo.Dial(os.Getenv(EnvDialURL))
	if err != nil {
		log.Fatalf("Failed to create a session: %v\n", err.Error())
	}
	defer session.Close()
	session.SetMode(mgo.Monotonic, true)

	err = session.Ping()
	if err != nil {
		log.Fatalf("Failed to ping DB: %v\n", err.Error())
	}

	//
	// Start the server
	//
	exitCh := make(chan bool, 1)
	go func() {
		//
		// Handler(s)
		//
		bucketHandler := NewBucketWebHandler()
		bucketHandler.Session = session
		objectHandler := NewObjectWebHandler()
		objectHandler.Session = session

		//
		// Middleware chain (mind the order!)
		//
		defaultChain := alice.New(
			context.ClearHandler, // cleanup ctx to avoid memory leakage
			LoggingHandler,       // basic requests logging
			RecoverHandler,       // transform panics into 500 responses
			InfoHeadersHandler,   // dummy handler to inject some info headers
		)
		jsonTypeHandler := NewContentTypeHandler("application/json")
		jsonChain := alice.New(
			context.ClearHandler, // cleanup ctx to avoid memory leakage
			LoggingHandler,       // basic requests logging
			RecoverHandler,       // transform panics into 500 responses
			jsonTypeHandler,      // check content-type for modification requests
			InfoHeadersHandler,   // dummy handler to inject some info headers
			JSONRenderingHandler, // always set JSON content-type for this API
		)

		//
		// Routing setup
		//
		router := newRouter()

		// Bucket API
		router.head("/buckets/:name", jsonChain.ThenFunc(bucketHandler.CheckExists))
		router.get("/buckets/:name", jsonChain.ThenFunc(bucketHandler.Retreive))
		router.delete("/buckets/:name", jsonChain.ThenFunc(bucketHandler.Delete))

		// Object API
		router.post("/objects", defaultChain.ThenFunc(objectHandler.Create))
		router.put("/objects/:id", jsonChain.ThenFunc(objectHandler.Update))
		router.head("/objects/:id", jsonChain.ThenFunc(objectHandler.CheckExists))
		router.get("/objects/:id", defaultChain.ThenFunc(objectHandler.Download))
		router.get("/objects/:id/meta", jsonChain.ThenFunc(objectHandler.About))
		router.delete("/objects/:id", jsonChain.ThenFunc(objectHandler.Delete))

		// Server setup
		addr := os.Getenv(EnvBindAddr)
		if addr == "" {
			addr = ":5000"
		}
		s := &http.Server{
			Addr:           addr,
			Handler:        router,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		// Create binding address listener
		listener, listenErr := net.Listen("tcp", addr)
		if listenErr != nil {
			log.Println("Could not listen: %s", listenErr)
			exitCh <- true
			return
		}
		defer listener.Close()
		log.Println("RESTful API Server listening", addr)
		serveErr := s.Serve(listener)
		if serveErr != nil {
			log.Println("Error in Serve:", serveErr)
			exitCh <- true
		}
	}()

	// Setup signal catcher for the server's proper shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	select {
	case s := <-c:
		log.Println("Caught signal", s.String())
	case <-exitCh:
		log.Println("Caught exit from one of the servers")
	}

	log.Println("Stopping the server...")
	// Tidy up and tear down
	log.Println("Tearing down...")
	log.Fatalln("Finished - bye bye. ;-)")
}
