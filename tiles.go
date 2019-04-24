package main

import (
	"bytes"
	"container/list"
	"sync"
	"sync/atomic"

	"github.com/fogleman/gg"
	"github.com/golang/geo/r2"
)

// Transparent 1x1 pixel PNG tile, 67 bytes
// http://garethrees.org/2007/11/14/pngcrush/
var emptyPNG []byte = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

type Tile struct {
	dc *gg.Context
}

func (t *Tile) DrawPoint(p r2.Point) {
	dc := t.dc
	if dc == nil {
		t.dc = gg.NewContext(256, 256)
		dc = t.dc
		dc.SetRGBA255(255, 255, 255, 0)
		dc.Clear()
		dc.SetRGB255(195, 66, 244)
	}
	dc.DrawCircle(p.X, p.Y, 2)
	dc.Fill()
}

func (t *Tile) ToPNG() []byte {
	if dc := t.dc; dc != nil {
		var png bytes.Buffer
		dc.EncodePNG(&png)
		return png.Bytes()
	} else {
		return emptyPNG
	}
}

type TileKey struct {
	X    uint32
	Y    uint32
	Zoom uint8
}

type TileCache struct {
	locks   [128]sync.Mutex
	lists   [128]list.List
	content [128]map[TileKey]*list.Element
	size    int32
	maxSize int32
}

type tileCacheEntry struct {
	key   TileKey
	value []byte
}

func NewTileCache(maxSize int32) *TileCache {
	tc := &TileCache{maxSize: maxSize}
	for i, _ := range tc.content {
		tc.content[i] = make(map[TileKey]*list.Element)
	}
	return tc
}

func getShard(key TileKey) int {
	return int((key.X ^ key.Y ^ (uint32(key.Zoom) << 4)) & 127)
}

func (tc *TileCache) Get(key TileKey) []byte {
	shard := getShard(key)
	tc.locks[shard].Lock()
	defer tc.locks[shard].Unlock()

	if e, hit := tc.content[shard][key]; hit {
		tc.lists[shard].MoveToFront(e)
		return e.Value.(*tileCacheEntry).value
	}

	return nil
}

func (tc *TileCache) Put(key TileKey, value []byte) {
	shard := getShard(key)
	tc.locks[shard].Lock()
	defer tc.locks[shard].Unlock()
	list := &tc.lists[shard]

	if e, hit := tc.content[shard][key]; hit {
		list.MoveToFront(e)
		e.Value.(*tileCacheEntry).value = value
		return
	}

	e := list.PushFront(&tileCacheEntry{key, value})
	tc.content[shard][key] = e
	if size := atomic.AddInt32(&tc.size, 1); size > tc.maxSize {
		if oldest := list.Back(); oldest != e {
			list.Remove(oldest)
			delete(tc.content[shard], oldest.Value.(*tileCacheEntry).key)
			atomic.AddInt32(&tc.size, -1)
		}
	}
}
