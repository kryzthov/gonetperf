package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

var (
	probes = make(map[string]*latencyProbe, 10)
)

func InitLatencyService() {
	http.HandleFunc("/latency/new", LatencyNewHandler)
	http.HandleFunc("/latency/stop", LatencyStopHandler)
	http.HandleFunc("/latency/status", LatencyStatusHandler)
	http.HandleFunc("/latency/series", LatencySeriesHandler)
}

// -------------------------------------------------------------------------------------------------

type LatencyNewRequest struct {
	Id         string `json:id`
	Target     string `json:target`
	IntervalMs int64  `json:intervalMs`
}

func LatencyNewHandler(w http.ResponseWriter, req *http.Request) {
	request := &LatencyNewRequest{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	intervalMs := request.IntervalMs
	if intervalMs == 0 {
		intervalMs = *flagDefaultIntervalMs
	}

	if _, exists := probes[request.Id]; exists {
		io.WriteString(w,
			fmt.Sprintf("Latency probe already exists for ID '%s'", request.Id))
		return
	}

	probe := NewLatencyProbe(request.Id, request.Target, intervalMs)
	probes[request.Id] = probe
	probe.Start()
}

// -------------------------------------------------------------------------------------------------

type LatencyStopRequest struct {
	Id string `json:id`
}

func LatencyStopHandler(w http.ResponseWriter, req *http.Request) {
	request := &LatencyStopRequest{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	if probe, exists := probes[request.Id]; exists {
		probe.Stop()
		probe.flush()
		delete(probes, request.Id)
		io.WriteString(w,
			fmt.Sprintf("Latency probe with ID '%s' stopped and removed", request.Id))
	} else {
		io.WriteString(w,
			fmt.Sprintf("No latency probe with ID '%s'.", request.Id))
	}
}

// -------------------------------------------------------------------------------------------------

type LatencyStatusRequest struct {
	// Id string `json:id`
}

func LatencyStatusHandler(w http.ResponseWriter, req *http.Request) {
	request := &LatencyNewRequest{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	for _, probe := range probes {
		io.WriteString(w,
			fmt.Sprintf("Latency to %s : %d µs\n", probe.id, probe.latency.Nanoseconds()/1000))
	}

	// probe, exists := probes[request.Id]
	// if !exists {
	// 	http.Error(w, fmt.Sprintf("No latency probe with ID '%s' not monitored", request.Id), 404)
	// 	return
	// }

	// io.WriteString(w, fmt.Sprintf("Target: %s\n", probe.target))
	// io.WriteString(w, fmt.Sprintf("Current: %d µs\n", int64(1000*1000*probe.current)))
	// io.WriteString(w, fmt.Sprintf("Min: %d µs\n", probe.minMicros))
	// io.WriteString(w, fmt.Sprintf("Max: %d µs\n", probe.maxMicros))
	// io.WriteString(w, fmt.Sprintf("Average: %.03f ms (var %.03f ms)\n",
	// 	probe.averageMs, probe.varianceMs))
}

// -------------------------------------------------------------------------------------------------

type LatencySeriesRequest struct {
	Id string `json:id`
}

func LatencySeriesHandler(w http.ResponseWriter, req *http.Request) {
	request := &LatencyNewRequest{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	probe, exists := probes[request.Id]
	if !exists {
		http.Error(w, fmt.Sprintf("No latency probe with ID '%s'", request.Id), 404)
		return
	}

	if file, err := os.Open(probe.logFilePath); err != nil {
		http.Error(w, fmt.Sprintf("Error opening log file '%s': %s", probe.logFilePath, err), 500)
		return
	} else {
		io.Copy(w, file)
	}
}
