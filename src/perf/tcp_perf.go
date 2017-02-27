package main

import (
	"flag"
	"fmt"
	"io"
	"net"

	"github.com/golang/glog"
)

var (
	flagTcpReadBufferSize = flag.Uint64("tcp-read-buffer-size", 16*1024,
		"Size of the buffer used when reading from a TCP connection.")
)

func handleTcpConnection(conn *net.TCPConn) {
	defer conn.Close()
	var buffer = make([]byte, *flagTcpReadBufferSize)

	var totalBytes = uint64(0)
	for {
		nbytes, err := conn.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			glog.Fatal("Error reading from TCP connection:", err)
		}
		glog.V(1).Infof("Received %d bytes from %s\n", nbytes, conn.RemoteAddr())
		totalBytes += uint64(nbytes)
	}
	glog.Info(
		"TCP connection terminated with ", totalBytes, " bytes received ",
		"from remote ", conn.RemoteAddr(), " and local ", conn.LocalAddr())
}

func startTcpService(port int) {
	addr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Fatal("Error resolving TCP address:", err)
	}
	if listener, err := net.ListenTCP("tcp", addr); err != nil {
		glog.Fatal("Error setting up listener for TCP connections:", err)
	} else {
		glog.Infof("Listening for TCP connections on %s\n", listener.Addr())
		for {
			if conn, err := listener.AcceptTCP(); err != nil {
				glog.Info("Error accepting TCP connection:", err)
			} else {
				glog.Info("Accepted TCP connection with remote ", conn.RemoteAddr(), " and local ", conn.LocalAddr())
				go handleTcpConnection(conn)
			}
		}
	}
}
