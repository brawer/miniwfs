package main

import (
	"encoding/json"
	//"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/paulmach/go.geojson"
	"io/ioutil"
	"log"
	"path/filepath"
	"sync"
	//"github.com/golang/geo/s2"
)

type Index struct {
	Collections map[string]*Collection
	mutex       sync.RWMutex
	watcher     *fsnotify.Watcher
}

type Collection struct {
	Features geojson.FeatureCollection
	Path     string
}

func MakeIndex(collections map[string]string) (*Index, error) {
	index := &Index{Collections: make(map[string]*Collection)}

	if watcher, err := fsnotify.NewWatcher(); err == nil {
		index.watcher = watcher
	} else {
		return nil, err
	}

	go index.watchFiles()
	for name, path := range collections {
		coll, err := ReadCollection(path)
		if err != nil {
			return nil, err
		}
		index.Collections[name] = coll
	}

	for _, c := range index.Collections {
		if err := index.watcher.Add(c.Path); err != nil {
			return nil, err
		}
	}

	return index, nil
}

func (index *Index) watchFiles() {
	for {
		select {
		case event, ok := <-index.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				if coll, err := ReadCollection(event.Name); err == nil {
					index.replaceCollection(coll)
				} else {
					log.Printf("error reading collection %s: %v",
						event.Name, err)
				}
			}
		}
	}
}

func (index *Index) replaceCollection(c *Collection) {
	index.mutex.Lock()
	defer index.mutex.Unlock()

	for name, old := range index.Collections {
		if c.Path == old.Path {
			index.Collections[name] = c
		}
	}
}

func ReadCollection(path string) (*Collection, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	coll := &Collection{Path: absPath}
	if err := json.Unmarshal(data, &coll.Features); err != nil {
		return nil, err
	}

	return coll, nil
}
