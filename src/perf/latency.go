package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/zorkian/go-datadog-api"
)

type Sample struct {
	// Unix time of the measurement
	timestamp uint64

	// Measured latency, in nanoseconds
	latencyNs uint64
}

type latencyProbe struct {
	id string

	// Time interval between measurements, in milliseconds
	intervalMs int64

	// Ticker with the time interval specified in `intervalMs`, or nil.
	ticker *time.Ticker

	series []Sample

	// Target HTTP URL to probe against
	target string

	// Most recent measurement
	latency time.Duration

	// Total number of measurements
	counter int64

	logFilePath string

	// Where to write samples
	logFile *os.File

	client *http.Client
}

func NewLatencyProbe(id, target string, intervalMs int64) *latencyProbe {
	bufferSize :=
		((time.Duration(1) * time.Minute) / (time.Duration(intervalMs) * time.Millisecond))

	probe := &latencyProbe{
		id:         id,
		target:     target,
		intervalMs: intervalMs,
		client: &http.Client{
			Timeout: time.Duration(intervalMs) * time.Millisecond,
		},
		series: make([]Sample, 0, bufferSize),
	}

	logFilePath := path.Join(*flagDataDir, fmt.Sprintf("%s.series", probe.id))
	glog.Infof("Writing latency measurements to %s\n", logFilePath)
	if file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640); err != nil {
		glog.Errorf("Error creating/opening log file %s : %s\n", logFilePath, err)
	} else {
		probe.logFilePath = logFilePath
		probe.logFile = file
	}
	return probe
}

func (p *latencyProbe) Start() {
	p.ticker = time.NewTicker(time.Duration(p.intervalMs) * time.Millisecond)
	go p.run(p.ticker)
}

func (p *latencyProbe) Stop() {
	p.ticker.Stop()
}

func (p *latencyProbe) getLatency() (time.Time, time.Duration, error) {
	startTime := time.Now()
	rep, err := p.client.Get(p.target)
	endTime := time.Now()
	latency := endTime.Sub(startTime)

	if rep != nil && rep.Body != nil {
		io.Copy(ioutil.Discard, rep.Body)
		rep.Body.Close()
	}

	// We identify a timeout by looking at the Get latency:
	if err != nil && latency < time.Duration(p.intervalMs)*time.Millisecond {
		return time.Time{}, 0, err
	}

	timestampNs := uint64(startTime.UnixNano())
	p.series = append(p.series, Sample{timestampNs, uint64(latency.Nanoseconds())})
	return startTime, latency, nil
}

func (p *latencyProbe) run(ticker *time.Ticker) {
	for {
		<-ticker.C
		p.counter += 1

		if timestamp, latency, err := p.getLatency(); err != nil {
			glog.Infof("Error while sending HTTP request for latency measurement: %s\n", err)
			continue
		} else {
			p.latency = latency

			metric := datadog.Metric{
				Metric: "network.p2p.latency",
				Points: []datadog.DataPoint{
					[2]float64{float64(timestamp.Unix()), latency.Seconds()},
				},
				Type: "gauge",
				Host: "",
				Tags: []string{
					fmt.Sprintf("source:%s", serverId), // source host
					fmt.Sprintf("target:%s", p.id),     // target host
				},
			}

			series := []datadog.Metric{metric}
			datadogClient.PostMetrics(series)
		}

		if len(p.series) >= cap(p.series) {
			p.flush()
		}
	}
}

func (p *latencyProbe) flush() {
	glog.V(1).Infof("Flushing %d samples to %s\n", len(p.series), p.logFilePath)
	samples := make([]string, len(p.series))
	for i, sample := range p.series {
		samples[i] = fmt.Sprintf("%d\t%d\n", sample.timestamp, sample.latencyNs)
	}
	p.logFile.WriteString(strings.Join(samples, ""))
	p.series = p.series[0:0]
}
