package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
)

type UdpReq struct {
	Target   string `json:target`
	MaxBytes uint64 `json:maxBytes`

	WriteSize       uint64 `json:writeSize`
	WriteIntervalMs uint64 `json:writeIntervalMs`

	// Optional start time (unix Epoch time, in seconds)
	StartTime uint64 `json:startTime`

	// Optional end time (unix Epoch time, in seconds)
	EndTime uint64 `json:endTime`
}

type UdpStopReq struct {
	Id string `json:id`
}

type UdpStatusReq struct {
	Id string `json:id`
}

type UdpRun struct {
	Id        string  `json:id`
	BytesSent uint64  `json:bytesSent`
	Req       *UdpReq `json:req`
	StopReq   bool

	// In UNIX nanoseconds
	TrafficStartTime int64 `json:trafficStartTime`
	TrafficEndTime   int64 `json:trafficEndTime`
}

var (
	udpRuns            = make(map[string]*UdpRun)
	udpRunCount uint64 = 0
)

func NewUdpRun(req *UdpReq) *UdpRun {
	if req.WriteSize == 0 {
		req.WriteSize = 1024
	}

	run := &UdpRun{}
	runId := atomic.AddUint64(&udpRunCount, 1) - 1
	run.Id = fmt.Sprintf("%s-%d", serverId, runId)
	run.Req = req

	udpRuns[run.Id] = run
	return run
}

func (run *UdpRun) Stop() {
	run.StopReq = true
}

func (run *UdpRun) waitForStartTime() {
	if run.Req.StartTime > 0 {
		startTime := time.Unix(int64(run.Req.StartTime), 0)
		glog.Infof("UDP traffic '%s' beginning in %.03f seconds",
			run.Id, startTime.Sub(time.Now()).Seconds())
		time.Sleep(startTime.Sub(time.Now()))
	}
}

func (run *UdpRun) getEndTime() (hasEndTime bool, endTime time.Time) {
	hasEndTime = (run.Req.EndTime > 0)
	if hasEndTime {
		endTime = time.Unix(int64(run.Req.EndTime), 0)
	}
	return
}

func (run *UdpRun) Process() {
	req := run.Req

	raddr, err := net.ResolveUDPAddr("udp4", req.Target)
	if err != nil {
		glog.Errorf("Error resolving UDP address '%s': %s\n", req.Target, err)
		return
	}

	var laddr *net.UDPAddr = nil
	conn, err := net.DialUDP("udp", laddr, raddr)
	if err != nil {
		glog.Errorf("Error opening socket to UDP target '%s': %s\n", req.Target, err)
		return
	}

	run.waitForStartTime()
	glog.Infof("Beginning UDP traffic '%s'", run.Id)

	hasEndTime, endTime := run.getEndTime()

	var data = make([]byte, req.WriteSize)
	run.TrafficStartTime = time.Now().UnixNano()
	var lastSendTime time.Time
	for {
		if run.StopReq {
			glog.Infof("Stopping UDP traffic run '%s'", run.Id)
			break
		}
		if (req.MaxBytes > 0) && (run.BytesSent >= req.MaxBytes) {
			glog.Infof("UDP traffic run '%s' completed (max bytes reached)", run.Id)
			break
		}
		if hasEndTime && time.Now().After(endTime) {
			glog.Infof("UDP traffic run '%s' completed (time is over)", run.Id)
			break
		}

		if req.WriteIntervalMs > 0 {
			var sleepTime = (time.Duration(req.WriteIntervalMs)*time.Millisecond -
				time.Since(lastSendTime))
			time.Sleep(sleepTime - time.Duration(1)*time.Millisecond)
		}
		lastSendTime = time.Now()
		var buffer = data[0:req.WriteSize]
		if req.MaxBytes > 0 {
			buffer = buffer[0:min(req.WriteSize, req.MaxBytes-run.BytesSent)]
		}
		nbytes, err := conn.Write(buffer)
		if err != nil {
			glog.Errorf("Error sending data over UDP to '%s': %s\n", req.Target, err)
			break
		}
		run.BytesSent += uint64(nbytes)
		glog.V(1).Infof("Sent %d bytes (%d out of %d bytes) from %s to %s over UDP",
			nbytes, run.BytesSent, req.MaxBytes, conn.LocalAddr(), raddr)
	}

	run.TrafficEndTime = time.Now().UnixNano()
	deltaNS := run.TrafficEndTime - run.TrafficStartTime
	glog.Infof("Completed UDP traffic request: %.03f b/s (%d bytes in %d ns)",
		float64(run.BytesSent)*1e9/float64(deltaNS), run.BytesSent, deltaNS)
}
