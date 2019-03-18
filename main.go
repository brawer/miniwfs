package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
)

func main() {
	collections := flag.String("collections", "castles=path/to/castles.geojson,lakes=path/to/lakes.geojson",
		"comma-separated list of collection=filepath, each being a GeoJSON feature collection that will be served to clients")
	flag.Parse()

	coll := make(map[string]string)
	for _, s := range strings.Split(*collections, ",") {
		p := strings.SplitN(s, "=", 2)
		if p == nil || len(p) != 2 {
			log.Fatal("malformed --collections command-line argument; pass something like --collections=castles=path/to/c.geojson,--lakes=path/to/l.geojson")
		}
		coll[p[0]] = p[1]
	}

	index, err := MakeIndex(coll)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("OK %v\n", index)
}
