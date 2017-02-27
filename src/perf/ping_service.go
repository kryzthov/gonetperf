package main

import (
	"github.com/golang/glog"
	"io"
	"net/http"
)

func PingHandler(w http.ResponseWriter, req *http.Request) {
	glog.V(1).Infof("Received %s request for URI %s from %s\n",
		req.Method, req.RequestURI, req.RemoteAddr)
	io.WriteString(w, "pong\n")
}

func InitPingService() {
	http.HandleFunc("/ping", PingHandler)
}
