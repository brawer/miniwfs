package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"

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

	index, err := MakeIndex(coll)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/metrics", promhttp.Handler())

	server := MakeWebServer(index, *publicPathPrefix)
	http.HandleFunc("/collections", server.HandleCollections)
	log.Printf("Listening for requests on port %v\n", strconv.Itoa(*port))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(*port), nil))
}
