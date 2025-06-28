package server

import (
	"fmt"
	"net/http"

	"github.com/ashupednekar/litefunctions/ingestor/pkg/broker"
)

// TODO: Error
// TODO: Settings

func (s *Server) SyncHandler(w http.ResponseWriter, r *http.Request){
	req, err := broker.Submit(s.nc, r)
	if err != nil{
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
	}
	res, err := broker.Reply(s.nc, req)
	if err != nil{
		http.Error(w, fmt.Sprintf("%s", err), http.StatusInternalServerError)
	}
	w.Write(res)
}

func (s *Server) SSEHandler(w http.ResponseWriter, r *http.Request){
	req, err := broker.Submit(s.nc, r)
	if err != nil{
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
	}
	ch, err := broker.Subscribe(s.nc, req)
	if err != nil{
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
	}
	for res := range ch{
		w.Write(res)
	}
}

func (s *Server) WSHandler(w http.ResponseWriter, r *http.Request){
	ch := make(chan []byte)
	req, err := broker.Produce(s.nc, ch, w, r)
	if err != nil{
		http.Error(w, fmt.Sprintf("%s", err), http.StatusInternalServerError)
	}
	ch, err = broker.Subscribe(s.nc, req)
	if err != nil{
		http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
	}
	for res := range ch{
		w.Write(res)
	}
}

