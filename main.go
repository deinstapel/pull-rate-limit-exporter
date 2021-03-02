package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dockerioRateLimitLimit = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dockerio_ratelimit_limit",
		Help: "Allowed pulls",
	})
	dockerioRateLimitRemaining = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dockerio_ratelimit_remaining",
		Help: "Remaining pulls",
	})
)

type Token struct {
	Token string `json:"token"`
}

func checkingRateLimit() {
	log.Print("Checking Rate Limit Status")
	r, err := http.Get("https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull")

	if err != nil {
		log.Print("Could not get token")
		return
	}

	var token Token
	err = json.NewDecoder(r.Body).Decode(&token)

	if err != nil {
		log.Print("Could not decode JSON from token request")
		return
	}

	req, err := http.NewRequest("HEAD", "https://registry-1.docker.io/v2/ratelimitpreview/test/manifests/latest", nil)
	if err != nil {
		log.Print("Could not create rate limit request")
		return
	}

	req.Header.Set("Authorization", "Bearer " + token.Token)
	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		log.Print("Could not get rate limit request")
		return
	}

	limit, err := strconv.ParseFloat(strings.Split(resp.Header.Get("RateLimit-Limit"), ";")[0], 64)
	remaining, err := strconv.ParseFloat(strings.Split(resp.Header.Get("RateLimit-Remaining"), ";")[0], 64)

	if err != nil {
		// Save a copy of this request for debugging.
		requestDump, _ := httputil.DumpResponse(resp, true)
		log.Print("Could not parse RateLimit headers")
		log.Print(string(requestDump))
		return
	}

	dockerioRateLimitLimit.Set(limit)
	dockerioRateLimitRemaining.Set(remaining)
}

func main() {

	ctx, cancelContext := context.WithCancel(context.Background())

	r := prometheus.NewRegistry()

	r.MustRegister(dockerioRateLimitLimit)
	r.MustRegister(dockerioRateLimitRemaining)

	ticker := time.NewTicker(10 * time.Second)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	checkingRateLimit()

	go func() {
		for {
			select {
			case <- ticker.C:
				checkingRateLimit()
			}
		}
	}()


	mux := http.NewServeMux()

	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	mux.Handle("/metrics", handler)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<head><title>Pull Rate Limit Exporter</title></head>
				<body>
				<h1>Pull Rate Limit Exporter</h1>
				<p><a href="/metrics">Metrics</a></p>
				</body>
			</html>
			`))
	})

	httpServer := &http.Server{
		Addr:        ":2342",
		Handler:     mux,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			// it is fine to use Fatal here because it is not main gorutine
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	shutdownCtx, cancelShutdownContext := context.WithTimeout(context.Background(), 5 * time.Second)

	for {
		select {
		case <-sigs:
			log.Print("Handling sigs")
			cancelContext()
			ticker.Stop()
			httpServer.RegisterOnShutdown(cancelShutdownContext)
			_ = httpServer.Shutdown(shutdownCtx)
			return
		}
	}

	os.Exit(0)
}