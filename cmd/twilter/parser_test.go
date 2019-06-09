package main

import (
	"github.com/kawasin73/twilter"
	"reflect"
	"testing"
)

func TestParseFilters(t *testing.T) {
	for _, test := range []struct {
		input  string
		output []twilter.Filter
	}{
		{"photo", []twilter.Filter{twilter.PhotoFilter{}}},
		{"video", []twilter.Filter{twilter.VideoFilter{}}},
		{"rt", []twilter.Filter{twilter.RTFilter{}}},
		{"qt", []twilter.Filter{twilter.QTFilter{}}},
		{"not(rt)", []twilter.Filter{twilter.NotFilter{twilter.RTFilter{}}}},
		{"and(rt,video,photo)", []twilter.Filter{
			twilter.AndFilter{
				[]twilter.Filter{twilter.RTFilter{}, twilter.VideoFilter{}, twilter.PhotoFilter{}},
			},
		}},
		{"or(rt,video,photo)", []twilter.Filter{
			twilter.OrFilter{
				[]twilter.Filter{twilter.RTFilter{}, twilter.VideoFilter{}, twilter.PhotoFilter{}},
			},
		}},
		{"and(rt,not(photo))/qt", []twilter.Filter{
			twilter.AndFilter{
				[]twilter.Filter{twilter.RTFilter{}, twilter.NotFilter{twilter.PhotoFilter{}}},
			},
			twilter.QTFilter{},
		}},
		{"and(qt,or(photo,and(video,photo)))", []twilter.Filter{
			twilter.AndFilter{
				[]twilter.Filter{twilter.QTFilter{}, twilter.OrFilter{
					[]twilter.Filter{twilter.PhotoFilter{}, twilter.AndFilter{
						[]twilter.Filter{twilter.VideoFilter{}, twilter.PhotoFilter{}},
					}},
				}},
			},
		}},
	} {
		filters, err := parseFilters(test.input, "/")
		if err != nil {
			t.Errorf("\"%v\" failed : %v", test.input, err)
		} else if !reflect.DeepEqual(filters, test.output) {
			t.Errorf("\"%v\" not equal : %v, expected %v", test.input, filters, test.output)
		}
	}
}
