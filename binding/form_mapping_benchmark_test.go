// Copyright 2019 Gin Core Team.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"testing"
	"time"
)

var form = map[string][]string{
	"name":      {"mike"},
	"friends":   {"anna", "nicole"},
	"id_number": {"12345678"},
	"id_date":   {"2018-01-20"},
}

type structFull struct {
	Name    string   `form:"name"`
	Age     int      `form:"age,default=25"`
	Friends []string `form:"friends"`
	ID      *struct {
		Number      string    `form:"id_number"`
		DateOfIssue time.Time `form:"id_date" time_format:"2006-01-02" time_utc:"true"`
	}
	Nationality *string `form:"nationality"`
}

func BenchmarkMapFormFull(b *testing.B) {
	var s structFull
	for i := 0; i < b.N; i++ {
		err := mapForm(&s, form)
		if err != nil {
			b.Fatalf("Error on a form mapping")
		}
	}
	b.StopTimer()
}

type structName struct {
	Name string `form:"name"`
}

func BenchmarkMapFormName(b *testing.B) {
	var s structName
	for i := 0; i < b.N; i++ {
		err := mapForm(&s, form)
		if err != nil {
			b.Fatalf("Error on a form mapping")
		}
	}
	b.StopTimer()
}
