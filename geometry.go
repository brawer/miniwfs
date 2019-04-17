package main

import (
	"github.com/golang/geo/s2"
	"github.com/paulmach/go.geojson"
	"math"
)

func computeBounds(g *geojson.Geometry) s2.Rect {
	r := s2.EmptyRect()
	if g == nil {
		return r
	}

	switch g.Type {
	case geojson.GeometryPoint:
		if len(g.Point) >= 2 {
			r = r.AddPoint(s2.LatLngFromDegrees(g.Point[1], g.Point[0]))
		}
		return r

	case geojson.GeometryMultiPoint:
		for _, p := range g.MultiPoint {
			if len(p) >= 2 {
				r = r.AddPoint(s2.LatLngFromDegrees(p[1], p[0]))
			}
		}
		return r

	case geojson.GeometryLineString:
		return computeLineBounds(g.LineString)

	case geojson.GeometryMultiLineString:
		for _, line := range g.MultiLineString {
			r = r.Union(computeLineBounds(line))
		}
		return r

	case geojson.GeometryPolygon:
		for _, ring := range g.Polygon {
			r = r.Union(computeLineBounds(ring))
		}
		s2.ExpandForSubregions(r)
		return r

	case geojson.GeometryMultiPolygon:
		for _, poly := range g.MultiPolygon {
			for _, ring := range poly {
				r = r.Union(computeLineBounds(ring))
			}
			s2.ExpandForSubregions(r)
		}
		return r

	case geojson.GeometryCollection:
		for _, geometry := range g.Geometries {
			r = r.Union(computeBounds(geometry))
		}
		return r

	default:
		return r
	}
}

func computeLineBounds(line [][]float64) s2.Rect {
	r := s2.EmptyRect()
	for _, p := range line {
		if len(p) >= 2 {
			r = r.AddPoint(s2.LatLngFromDegrees(p[1], p[0]))
		}
	}
	return r
}

func EncodeBbox(r s2.Rect) []float64 {
	if r.IsEmpty() {
		return nil
	} else {
		bbox := [4]float64{
			r.Lo().Lng.Degrees(),
			r.Lo().Lat.Degrees(),
			r.Hi().Lng.Degrees(),
			r.Hi().Lat.Degrees(),
		}
		return bbox[0:4]
	}
}

func getTileBounds(zoom int, x int, y int) s2.Rect {
	r := s2.RectFromLatLng(projectFromEPSG3857(zoom, float64(x), float64(y)))
	return r.AddPoint(projectFromEPSG3857(zoom, float64(x+1), float64(y+1)))
}

func projectFromEPSG3857(zoom int, x float64, y float64) s2.LatLng {
	// EPSG:3857 - https://epsg.io/3857
	n := math.Pi - 2.0*math.Pi*y/math.Exp2(float64(zoom))
	lat := 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	lng := x/math.Exp2(float64(zoom))*360.0 - 180.0
	return s2.LatLngFromDegrees(lat, lng)
}
