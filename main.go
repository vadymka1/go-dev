package go_dev

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

type Server struct {
	http.Server
	shutdownReq chan bool
	reqCount uint32
}

var port = ":8080"
var word string

func main() {
	server := StartServer()

	done := make(chan bool)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Printf("Listen and serve: %v", err)
		}
		log.Println("Server starting")
		done <- true
	}()

	server.WaitShutDown()

	<-done
	log.Printf("DONE!")
}

func StartServer() *Server {
	srv := &Server{
		Server:      http.Server{
			Addr         : port,
			ReadTimeout  : 10*time.Second,
			WriteTimeout : 10*time.Second,
		},
		shutdownReq: make(chan bool),
		reqCount:    0,
	}

	router := mux.NewRouter()
	router.HandleFunc("/shutdown", srv.ShutdownHandler)
	router.HandleFunc("api/get", Get).Methods("GET")
	router.HandleFunc("api/post", Post).Methods("Post")

	srv.Handler = router

	return srv
}

func Get (w http.ResponseWriter, r* http.Request) {
	query := r.URL.Query()
	name := query.Get("name")
	if name == "" {
		name = "Guest"
	}
	log.Printf("Received request for %s\n", name)
	w.Write([]byte(fmt.Sprintf("Hello, %s\n", name)))
}

func Post (w http.ResponseWriter, r* http.Request) {
	resp := Message(true, "Success")
	resp["data"] = "POST"
	Respond(w, resp)
}


func Message(status bool, message string) map[string]interface{} {
	return map[string]interface{}{"status": status, "message": message}
}

/** Respond ...**/
func Respond(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) WaitShutDown() {
	irqSig := make(chan os.Signal, 1)
	signal.Notify(irqSig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-irqSig:
		log.Printf("Shutdown request (signal: %v", sig)
	case sig := <-s.shutdownReq:
		log.Printf("Shutdown request (/shutdown %v)", sig)
	}

	log.Println("Stopping http server..")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	err := s.Shutdown(ctx)
	if err != nil {
		log.Printf("Shutdown request error: %v", err)
	}

}

func (s *Server) ShutdownHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Shutdown server"))

	if !atomic.CompareAndSwapUint32(&s.reqCount, 0, 1) {
		log.Printf("Shutdown through API call in progress...")
		return
	}

	go func() {
		s.shutdownReq <- true
	}()
}