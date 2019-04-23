package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	collections := flag.String("collections", "castles=path/to/castles.geojson,lakes=path/to/lakes.geojson",
		"comma-separated list of collection=filepath, each being a GeoJSON feature collection that will be served to clients")
	port := flag.Int("port", 8080, "TCP port for serving requests")
	publicPathPrefix := flag.String("pathPrefix", "http://localhost:8080/",
		"externally accessible http path to this server")
	flag.Parse()

	coll := make(map[string]string)
	for _, s := range strings.Split(*collections, ",") {
		p := strings.SplitN(s, "=", 2)
		if p == nil || len(p) != 2 {
			log.Fatal("malformed --collections command-line argument; pass something like --collections=castles=path/to/c.geojson,lakes=path/to/l.geojson")
		}
		coll[p[0]] = p[1]
	}

	publicPath, err := url.Parse(*publicPathPrefix)
	if err != nil {
		log.Fatal(err)
	}

	index, err := MakeIndex(coll, publicPath)
	if err != nil {
		log.Fatal(err)
	}
	defer index.Close()

	server := MakeWebServer(index)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/collections", server.HandleRequest)
	http.HandleFunc("/collections/", server.HandleRequest)
	http.HandleFunc("/tiles/", server.HandleRequest)
	log.Printf("Listening for requests on port %v\n", strconv.Itoa(*port))
	go func() { // Gracefully shut down server upon SIGINT, so we do not lose queries.
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		server.Shutdown()
	}()
	if err := server.ListenAndServe(*port); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Printf("Server has shut down.\n")
}
