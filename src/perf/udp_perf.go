package main

import (
	"flag"
	"fmt"
	"io"
	"net"

	"github.com/golang/glog"
)

var (
	flagUdpReadBufferSize = flag.Uint64("udp-read-buffer-size", 16*1024,
		"Size of the buffer used when reading UDP messages.")
)

func handleUdpMessages(conn *net.UDPConn) {
	defer conn.Close()
	var buffer = make([]byte, *flagUdpReadBufferSize)

	var totals = make(map[string]uint64)
	var totalBytes = uint64(0)
	for {
		nbytes, remoteAddr, err := conn.ReadFrom(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			glog.Fatal("Error reading from UDP socket:", err)
		}
		var raddr = remoteAddr.String()
		total := totals[raddr]
		total += uint64(nbytes)
		totals[raddr] = total
		glog.Infof("Received %d bytes over UDP from %s (total = %d)\n", nbytes, raddr, total)
		totalBytes += uint64(nbytes)
	}
}

func startUdpService(port int) {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Fatal("Error resolving UDP address:", err)
	}
	if conn, err := net.ListenUDP("udp", addr); err != nil {
		glog.Fatal("Error setting up UDP service:", err)
	} else {
		glog.Info("UDP service ready on local address:", conn.LocalAddr())
		handleUdpMessages(conn)
	}
}
