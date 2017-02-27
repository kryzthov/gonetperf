package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
)

type TcpReq struct {
	Target   string `json:target`
	MaxBytes uint64 `json:maxBytes`

	WriteSize       uint64 `json:writeSize`
	WriteIntervalMs uint64 `json:writeIntervalMs`

	// Optional start time (unix Epoch time, in seconds)
	StartTime uint64 `json:startTime`

	// Optional end time (unix Epoch time, in seconds)
	EndTime uint64 `json:endTime`
}

type TcpStopReq struct {
	Id string `json:id`
}

type TcpStatusReq struct {
	Id string `json:id`
}

type TcpRun struct {
	Id        string  `json:id`
	BytesSent uint64  `json:bytesSent`
	Req       *TcpReq `json:req`
	StopReq   bool

	// In UNIX nanoseconds
	TrafficStartTime int64 `json:trafficStartTime`
	TrafficEndTime   int64 `json:trafficEndTime`
}

var (
	tcpRuns            = make(map[string]*TcpRun)
	tcpRunCount uint64 = 0
)

func NewTcpRun(req *TcpReq) *TcpRun {
	if req.WriteSize == 0 {
		req.WriteSize = 1024
	}

	run := &TcpRun{}
	runId := atomic.AddUint64(&tcpRunCount, 1) - 1
	run.Id = fmt.Sprintf("%s-%d", serverId, runId)
	run.Req = req

	tcpRuns[run.Id] = run
	return run
}

func (run *TcpRun) Stop() {
	run.StopReq = true
}

func (run *TcpRun) waitForStartTime() {
	if run.Req.StartTime > 0 {
		startTime := time.Unix(int64(run.Req.StartTime), 0)
		glog.Infof("TCP traffic '%s' beginning in %.03f seconds",
			run.Id, startTime.Sub(time.Now()).Seconds())
		time.Sleep(startTime.Sub(time.Now()))
	}
}

func (run *TcpRun) getEndTime() (hasEndTime bool, endTime time.Time) {
	hasEndTime = (run.Req.EndTime > 0)
	if hasEndTime {
		endTime = time.Unix(int64(run.Req.EndTime), 0)
	}
	return
}

func (run *TcpRun) Process() {
	req := run.Req
	bufferSize := req.WriteSize

	time0 := time.Now()
	conn, err := net.Dial("tcp", req.Target)
	if err != nil {
		glog.Errorf("Error connecting to TCP target '%s': %s\n", req.Target, err)
		return
	}
	time1 := time.Now()
	defer conn.Close()
	glog.Infof("Established connection to TCP target '%s' from %s to %s\n",
		req.Target, conn.LocalAddr(), conn.RemoteAddr())

	run.waitForStartTime()
	glog.Infof("Beginning TCP traffic '%s'", run.Id)

	hasEndTime, endTime := run.getEndTime()

	var data = make([]byte, bufferSize)
	run.TrafficStartTime = time.Now().UnixNano()
	var lastSendTime time.Time
	for {
		if run.StopReq {
			glog.Infof("Stopping TCP traffic run '%s'", run.Id)
			break
		}
		if (req.MaxBytes > 0) && (run.BytesSent >= req.MaxBytes) {
			glog.Infof("TCP traffic run '%s' completed (max bytes reached)", run.Id)
			break
		}
		if hasEndTime && time.Now().After(endTime) {
			glog.Infof("TCP traffic run '%s' completed (time is over)", run.Id)
			break
		}

		if req.WriteIntervalMs > 0 {
			var sleepTime = (time.Duration(req.WriteIntervalMs)*time.Millisecond -
				time.Since(lastSendTime))
			time.Sleep(sleepTime - time.Duration(1)*time.Millisecond)
		}
		lastSendTime = time.Now()
		var buffer = data[0:bufferSize]
		if req.MaxBytes > 0 {
			buffer = buffer[0:min(bufferSize, req.MaxBytes-run.BytesSent)]
		}
		nbytes, err := conn.Write(buffer)
		if err != nil {
			glog.Errorf("Error sending data over TCP to '%s': %s\n", req.Target, err)
			break
		}
		run.BytesSent += uint64(nbytes)
		glog.V(1).Infof("Sent %d bytes (%d out of %d bytes) from %s to %s over UDP",
			nbytes, run.BytesSent, req.MaxBytes, conn.LocalAddr(), conn.RemoteAddr())
	}

	run.TrafficEndTime = time.Now().UnixNano()
	deltaNS := run.TrafficEndTime - run.TrafficStartTime
	glog.Infof("Established TCP connection in %d ns", time1.Sub(time0).Nanoseconds())
	glog.Infof("Completed TCP traffic request: %.03f b/s (%d bytes in %d ns)",
		float64(run.BytesSent)*1e9/float64(deltaNS), run.BytesSent, deltaNS)
}
