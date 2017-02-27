package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/zorkian/go-datadog-api"
)

func defaultId() string {
	id := os.Getenv("K8S_POD_NAME")
	if len(id) > 0 {
		return id
	}
	id, err := os.Hostname()
	if err != nil {
		fmt.Printf("Unable to get local hostname: %v\n", err)
	}
	return id
}

var (
	httpPort = flag.Int("http-port", 80, "Port to listen on for HTTP requests.")
	tcpPort  = flag.Int("tcp-port", 4000, "Port to listen on for TCP traffic.")
	udpPort  = flag.Int("udp-port", 5000, "Port to listen on for UDP traffic.")

	flagPsName        = flag.String("petset-name", "", "Pet-set/service name.")
	flagNsName        = flag.String("ns-name", os.Getenv("K8S_NAMESPACE"), "Namespace name.")
	flagClusterDomain = flag.String("cluster-domain", "cluster.test", "DNS domain of the cluster.")

	flagDefaultIntervalMs = flag.Int64("default-interval-ms", 1000,
		"Default time interval in between latency measurements")

	flagDataDir = flag.String("data-dir", os.Getenv("PWD"),
		"Directory where to write samples data files.")

	flagId = flag.String("id", defaultId(), "ID of this server.")

	flagDatadogApiKey = flag.String("datadog-api-key", "", "Datadog API key")
	flagDatadogAppKey = flag.String("datadog-app-key", "", "Datadog application key")
	flagDatadogUrl    = flag.String("datadog-url", "http://localhost:17123",
		"URL of the Datadog API endpoint")
)

// -------------------------------------------------------------------------------------------------

var (
	serverId      string
	datadogClient *datadog.Client
)

func ParseRequest(w http.ResponseWriter, req *http.Request, request interface{}) error {
	glog.V(1).Infof("Received '%s' request from %s\n", req.RequestURI, req.RemoteAddr)

	data := make([]byte, req.ContentLength)
	if _, err := io.ReadFull(req.Body, data); err != nil {
		msg := fmt.Sprintf("Error reading body for '%s' request: %s\n", req.RequestURI, err)
		glog.Error(msg)
		http.Error(w, msg, 500)
		return err
	}

	glog.Infof("Received '%s' request with body '%s'", req.RequestURI, string(data))
	if err := json.Unmarshal(data, request); err != nil {
		msg := fmt.Sprintf("Error decoding JSON body for '%s' request: %s\n", req.RequestURI, err)
		glog.Error(msg)
		http.Error(w, msg, 500)
		return err
	}

	glog.V(1).Infof("Parsed request for '%s': %v\n", req.RequestURI, request)
	return nil
}

func WriteReply(w http.ResponseWriter, req *http.Request, reply interface{}) error {
	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	if data, err := json.Marshal(reply); err != nil {
		msg := fmt.Sprintf("Error serializing reply for '%s': %s", req.RequestURI, err)
		glog.Error(msg)
		http.Error(w, msg, 500)
		return err
	} else if nbytes, err := w.Write(data); (err != nil) || (nbytes != len(data)) {
		msg := fmt.Sprintf("Error serializing reply for '%s': %s", req.RequestURI, err)
		glog.Error(msg)
		http.Error(w, msg, 500)
		return err
	}
	return nil
}

func TcpHandler(w http.ResponseWriter, req *http.Request) {
	request := &TcpReq{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	run := NewTcpRun(request)
	if err := WriteReply(w, req, run); err != nil {
		return
	}
	go run.Process()
}

func TcpStopHandler(w http.ResponseWriter, req *http.Request) {
	request := &TcpStopReq{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	if run, ok := tcpRuns[request.Id]; ok {
		run.Stop()
	} else {
		http.Error(w, fmt.Sprintf("No TCP run with ID '%s'", request.Id), 404)
	}
}

func TcpStatusHandler(w http.ResponseWriter, req *http.Request) {
	request := &TcpStatusReq{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	if run, ok := tcpRuns[request.Id]; ok {
		WriteReply(w, req, run)
	} else {
		http.Error(w, fmt.Sprintf("No TCP run with ID '%s'", request.Id), 404)
	}
}

func UdpHandler(w http.ResponseWriter, req *http.Request) {
	request := &UdpReq{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	run := NewUdpRun(request)
	if err := WriteReply(w, req, run); err != nil {
		return
	}
	go run.Process()
}

func UdpStopHandler(w http.ResponseWriter, req *http.Request) {
	request := &UdpStopReq{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	if run, ok := udpRuns[request.Id]; ok {
		run.Stop()
	} else {
		http.Error(w, fmt.Sprintf("No UDP run with ID '%s'", request.Id), 404)
	}
}

func UdpStatusHandler(w http.ResponseWriter, req *http.Request) {
	request := &UdpStatusReq{}
	if err := ParseRequest(w, req, request); err != nil {
		return
	}

	if run, ok := udpRuns[request.Id]; ok {
		WriteReply(w, req, run)
	} else {
		http.Error(w, fmt.Sprintf("No UDP run with ID '%s'", request.Id), 404)
	}
}

func startHttpService(port int) {
	http.HandleFunc("/tcp/stop", TcpStopHandler)
	http.HandleFunc("/tcp/status", TcpStatusHandler)
	http.HandleFunc("/tcp", TcpHandler)

	http.HandleFunc("/udp/stop", UdpStopHandler)
	http.HandleFunc("/udp/status", UdpStatusHandler)
	http.HandleFunc("/udp", UdpHandler)

	address := fmt.Sprintf(":%d", port)
	glog.Infof("Starting ping/pong service on %s\n", address)
	glog.Fatal(http.ListenAndServe(address, nil))
}

// -------------------------------------------------------------------------------------------------

// Read configuration of a PetSet deployment
func readConfig(psName, nsName string) (int, string, []int) {
	var name string
	var err error
	if name, err = os.Hostname(); err != nil {
		glog.Fatal("Error getting hostname:", err)
	}

	split := strings.SplitN(name, "-", 2)
	if psName == "" {
		psName = split[0]
		glog.Infof("Using petset/service name '%s'\n", psName)
	}
	index, err := strconv.Atoi(split[1])
	if split[0] != psName || err != nil {
		glog.Fatalf("Improper host name, expecting '%s-index' but got '%s'.", psName, name)
	}

	srvName := fmt.Sprintf("%s.%s.svc.%s", psName, nsName, *flagClusterDomain)
	_, addrs, err := net.LookupSRV("", "", srvName)
	if err != nil {
		glog.Fatal("Error looking up SRV record:", err)
	}
	var memberIds []int = make([]int, 0, len(addrs))
	for _, addr := range addrs {
		memberName := strings.SplitN(addr.Target, ".", 2)[0]
		split = strings.SplitN(memberName, "-", 2)
		memberId, err := strconv.Atoi(split[1])
		if psName != split[0] || err != nil {
			glog.Fatalf("Improper member name, expecting '%s-index' but got '%s'.",
				psName, memberName)
		}
		memberIds = append(memberIds, memberId)
	}
	return index, psName, memberIds
}

// -------------------------------------------------------------------------------------------------

func main() {
	flag.Parse()
	serverId = *flagId
	glog.Infof("Initialized server with ID '%s'", serverId)
	glog.Infof("Writing data files to '%s'\n", *flagDataDir)

	os.Setenv("DATADOG_HOST", *flagDatadogUrl)
	datadogClient = datadog.NewClient(*flagDatadogApiKey, *flagDatadogAppKey)

	InitPingService()
	InitLatencyService()

	go startHttpService(*httpPort)
	go startTcpService(*tcpPort)
	go startUdpService(*udpPort)

	select {} // wait forever
}
