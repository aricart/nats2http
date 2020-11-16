package main

import (
	"fmt"
	"net"
	"net/http"
	"runtime"

	"github.com/aricart/nats2http"
	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats.go"
)

const HostPort = "127.0.0.1:8080"

func main() {
	_, err := NewServer()
	if err != nil {
		panic(err)
	}
	runtime.Goexit()
}

type Server struct {
	httpSrv *http.Server
	nc      *nats.Conn
}

func NewServer() (*Server, error) {
	s := &Server{}
	if err := s.startHTTP(); err != nil {
		return nil, err
	}
	if err := s.startNATS(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) startNATS() error {
	var err error
	s.nc, err = nats.Connect("localhost:4222")
	if err != nil {
		return err
	}

	a := nats2http.HttpServiceAdapter{BaseURL: fmt.Sprintf("http://%s", HostPort), HttpHandler: s.httpSrv.Handler}
	s.nc.Subscribe(">", a.NatsHandler())
	return err
}

func (s *Server) startHTTP() error {
	router, err := buildRouter()
	if err != nil {
		return err
	}
	listen, err := net.Listen("tcp", HostPort)
	if err != nil {
		return err
	}
	s.httpSrv = &http.Server{
		Handler: router,
	}
	go func() {
		if err := s.httpSrv.Serve(listen); err != nil {
			panic(err)
		}
	}()

	return nil
}

func buildRouter() (*httprouter.Router, error) {
	r := httprouter.New()
	r.GET("/dowork", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		fmt.Println("/dowork")
		w.WriteHeader(200)
		w.Write([]byte("OK!"))
	})

	return r, nil
}
