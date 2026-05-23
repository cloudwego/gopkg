/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package binding

import (
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecoder_Base(t *testing.T) {
	type TestStruct struct {
		A bool    `query:"a"`
		B uint    `query:"b"`
		C uint8   `query:"c"`
		D uint16  `query:"d"`
		E uint32  `query:"e"`
		F uint64  `query:"f"`
		G int     `query:"g"`
		H int8    `query:"h"`
		I int16   `query:"i"`
		J int32   `query:"j"`
		K int64   `query:"k"`
		L string  `query:"l"`
		M float32 `query:"m"`
		N float64 `query:"n"`
	}

	ctx := newTestContextWithQuery("a=1&b=2&c=3&d=4&e=5&f=6&g=7&h=8&i=9&j=10&k=11&l=12&m=13&n=14")
	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.Nil(t, err, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.True(t, changed)
	assert.NoError(t, err)
	assert.True(t, p.A)
	assert.Equal(t, uint(2), p.B)
	assert.Equal(t, uint8(3), p.C)
	assert.Equal(t, uint16(4), p.D)
	assert.Equal(t, uint32(5), p.E)
	assert.Equal(t, uint64(6), p.F)
	assert.Equal(t, 7, p.G)
	assert.Equal(t, int8(8), p.H)
	assert.Equal(t, int16(9), p.I)
	assert.Equal(t, int32(10), p.J)
	assert.Equal(t, int64(11), p.K)
	assert.Equal(t, "12", p.L)
	assert.Equal(t, float32(13), p.M)
	assert.Equal(t, float64(14), p.N)
}

func TestDecoder_Map(t *testing.T) {
	type TestStruct struct {
		M map[string]string `query:"m"`
	}

	ctx := newTestContextWithQuery(`m=%7B%22x%22%3A%22y%22%7D`)

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.Nil(t, err, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.True(t, changed)
	assert.NoError(t, err)
	assert.Len(t, p.M, 1)
	assert.Equal(t, "y", p.M["x"])
}

func TestDecoder_File(t *testing.T) {
	type TestStruct struct {
		F1 *multipart.FileHeader   `form:"test1.go" file_name:"test2.go"`
		F2 multipart.FileHeader    `form:"test1.go"`
		F3 []*multipart.FileHeader `form:"test1.go" file_name:"test3.go"`
	}

	f1 := &multipart.FileHeader{Filename: "test1.go"}
	f2 := &multipart.FileHeader{Filename: "test2.go"}
	f3 := &multipart.FileHeader{Filename: "test3-1.go"}
	f4 := &multipart.FileHeader{Filename: "test3-2.go"}

	req := httptest.NewRequest("POST", "http://example.com/", nil)
	req.MultipartForm = &multipart.Form{
		File: map[string][]*multipart.FileHeader{
			"test1.go": {f1},
			"test2.go": {f2},
			"test3.go": {f3, f4},
		},
	}
	ctx := newTestContextFromRequest(req)

	c := &DecodeConfig{Tags: []string{"form", "file_name"}}
	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), c)
	assert.Nil(t, err, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.True(t, changed)
	assert.NoError(t, err)
	assert.NotNil(t, p.F1)
	assert.Equal(t, f2.Filename, p.F1.Filename)
	assert.Equal(t, f1.Filename, p.F2.Filename)
	assert.Len(t, p.F3, 2)
	assert.Equal(t, f3.Filename, p.F3[0].Filename)
	assert.Equal(t, f4.Filename, p.F3[1].Filename)
}

func TestDecoder_Slice(t *testing.T) {
	type TestStruct struct {
		// Strings
		SS []string `query:"ss"`
		// Signed ints
		SI   []int   `query:"si"`
		SI8  []int8  `query:"si8"`
		SI16 []int16 `query:"si16"`
		SI32 []int32 `query:"si32"`
		SI64 []int64 `query:"si64"`
		// Unsigned ints
		SU   []uint   `query:"su"`
		SU8  []uint8  `query:"su8"`
		SU16 []uint16 `query:"su16"`
		SU32 []uint32 `query:"su32"`
		SU64 []uint64 `query:"su64"`
		// Floats
		SF32 []float32 `query:"sf32"`
		SF64 []float64 `query:"sf64"`
		// Bools
		SB []bool `query:"sb"`
	}

	ctx := newTestContextWithQuery(
		"ss=1&ss=2&" +
			"si=1&si=-2&si=3&si8=4&si8=-5&si16=6&si16=-7&si32=8&si32=-9&si64=10&si64=-11&" +
			"su=1&su=2&su=3&su8=4&su8=5&su16=6&su16=7&su32=8&su32=9&su64=10&su64=11&" +
			"sf32=1.5&sf32=-2.5&sf32=3.14&sf64=4.56&sf64=-7.89&" +
			"sb=true&sb=false&sb=1&sb=0&sb=T&sb=F")

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.NoError(t, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.True(t, changed)
	assert.NoError(t, err)

	// Strings
	assert.Len(t, p.SS, 2)
	assert.Equal(t, "1", p.SS[0])
	assert.Equal(t, "2", p.SS[1])

	// Signed ints
	assert.Len(t, p.SI, 3)
	assert.Equal(t, 1, p.SI[0])
	assert.Equal(t, -2, p.SI[1])
	assert.Equal(t, 3, p.SI[2])

	assert.Len(t, p.SI8, 2)
	assert.Equal(t, int8(4), p.SI8[0])
	assert.Equal(t, int8(-5), p.SI8[1])

	assert.Len(t, p.SI16, 2)
	assert.Equal(t, int16(6), p.SI16[0])
	assert.Equal(t, int16(-7), p.SI16[1])

	assert.Len(t, p.SI32, 2)
	assert.Equal(t, int32(8), p.SI32[0])
	assert.Equal(t, int32(-9), p.SI32[1])

	assert.Len(t, p.SI64, 2)
	assert.Equal(t, int64(10), p.SI64[0])
	assert.Equal(t, int64(-11), p.SI64[1])

	// Unsigned ints
	assert.Len(t, p.SU, 3)
	assert.Equal(t, uint(1), p.SU[0])
	assert.Equal(t, uint(2), p.SU[1])
	assert.Equal(t, uint(3), p.SU[2])

	assert.Len(t, p.SU8, 2)
	assert.Equal(t, uint8(4), p.SU8[0])
	assert.Equal(t, uint8(5), p.SU8[1])

	assert.Len(t, p.SU16, 2)
	assert.Equal(t, uint16(6), p.SU16[0])
	assert.Equal(t, uint16(7), p.SU16[1])

	assert.Len(t, p.SU32, 2)
	assert.Equal(t, uint32(8), p.SU32[0])
	assert.Equal(t, uint32(9), p.SU32[1])

	assert.Len(t, p.SU64, 2)
	assert.Equal(t, uint64(10), p.SU64[0])
	assert.Equal(t, uint64(11), p.SU64[1])

	// Floats
	assert.Len(t, p.SF32, 3)
	assert.InDelta(t, float32(1.5), p.SF32[0], 0.001)
	assert.InDelta(t, float32(-2.5), p.SF32[1], 0.001)
	assert.InDelta(t, float32(3.14), p.SF32[2], 0.001)

	assert.Len(t, p.SF64, 2)
	assert.InDelta(t, 4.56, p.SF64[0], 0.001)
	assert.InDelta(t, -7.89, p.SF64[1], 0.001)

	// Bools
	assert.Len(t, p.SB, 6)
	assert.Equal(t, true, p.SB[0])
	assert.Equal(t, false, p.SB[1])
	assert.Equal(t, true, p.SB[2])
	assert.Equal(t, false, p.SB[3])
	assert.Equal(t, true, p.SB[4])
	assert.Equal(t, false, p.SB[5])
}

func TestDecoder_Array(t *testing.T) {
	type TestStruct struct {
		// Signed ints
		AI   [3]int   `query:"ai"`
		AI8  [2]int8  `query:"ai8"`
		AI16 [2]int16 `query:"ai16"`
		AI32 [2]int32 `query:"ai32"`
		AI64 [2]int64 `query:"ai64"`
		// Unsigned ints
		AU   [2]uint   `query:"au"`
		AU8  [3]uint8  `query:"au8"`
		AU16 [2]uint16 `query:"au16"`
		AU32 [2]uint32 `query:"au32"`
		AU64 [2]uint64 `query:"au64"`
		// Floats
		AF32 [2]float32 `query:"af32"`
		AF64 [3]float64 `query:"af64"`
		// Bools
		AB [4]bool `query:"ab"`
	}

	ctx := newTestContextWithQuery(
		"ai=1&ai=2&ai=3&ai8=4&ai8=-5&ai16=6&ai16=-7&ai32=8&ai32=-9&ai64=10&ai64=-11&" +
			"au=1&au=2&au8=3&au8=4&au8=5&au16=6&au16=7&au32=8&au32=9&au64=10&au64=11&" +
			"af32=1.5&af32=2.5&af64=3.14&af64=4.56&af64=5.78&" +
			"ab=true&ab=false&ab=1&ab=0")

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.NoError(t, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.True(t, changed)
	assert.NoError(t, err)

	// Signed ints
	assert.Equal(t, 1, p.AI[0])
	assert.Equal(t, 2, p.AI[1])
	assert.Equal(t, 3, p.AI[2])
	assert.Equal(t, int8(4), p.AI8[0])
	assert.Equal(t, int8(-5), p.AI8[1])
	assert.Equal(t, int16(6), p.AI16[0])
	assert.Equal(t, int16(-7), p.AI16[1])
	assert.Equal(t, int32(8), p.AI32[0])
	assert.Equal(t, int32(-9), p.AI32[1])
	assert.Equal(t, int64(10), p.AI64[0])
	assert.Equal(t, int64(-11), p.AI64[1])

	// Unsigned ints
	assert.Equal(t, uint(1), p.AU[0])
	assert.Equal(t, uint(2), p.AU[1])
	assert.Equal(t, uint8(3), p.AU8[0])
	assert.Equal(t, uint8(4), p.AU8[1])
	assert.Equal(t, uint8(5), p.AU8[2])
	assert.Equal(t, uint16(6), p.AU16[0])
	assert.Equal(t, uint16(7), p.AU16[1])
	assert.Equal(t, uint32(8), p.AU32[0])
	assert.Equal(t, uint32(9), p.AU32[1])
	assert.Equal(t, uint64(10), p.AU64[0])
	assert.Equal(t, uint64(11), p.AU64[1])

	// Floats
	assert.InDelta(t, float32(1.5), p.AF32[0], 0.001)
	assert.InDelta(t, float32(2.5), p.AF32[1], 0.001)
	assert.InDelta(t, 3.14, p.AF64[0], 0.001)
	assert.InDelta(t, 4.56, p.AF64[1], 0.001)
	assert.InDelta(t, 5.78, p.AF64[2], 0.001)

	// Bools
	assert.Equal(t, true, p.AB[0])
	assert.Equal(t, false, p.AB[1])
	assert.Equal(t, true, p.AB[2])
	assert.Equal(t, false, p.AB[3])
}

func TestDecoder_SliceErrors(t *testing.T) {
	type TestStruct struct {
		SI []int     `query:"si"`
		SF []float64 `query:"sf"`
		SU []uint    `query:"su"`
		SB []bool    `query:"sb"`
	}

	tests := []struct {
		name  string
		query string
	}{
		{"invalid int", "si=abc"},
		{"invalid float", "sf=notanumber"},
		{"negative unsigned int", "su=-5"},
		{"invalid bool", "sb=maybe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContextWithQuery(tt.query)
			dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
			assert.NoError(t, err)

			p := &TestStruct{}
			_, err = dec.Decode(ctx, p)
			assert.Error(t, err)
		})
	}
}

func TestDecoder_ArrayLengthMismatch(t *testing.T) {
	type TestStruct struct {
		AI [2]int `query:"ai"`
	}

	// Provide 3 values for array of length 2
	ctx := newTestContextWithQuery("ai=1&ai=2&ai=3")
	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.NoError(t, err)

	p := &TestStruct{}
	_, err = dec.Decode(ctx, p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not valid value")
}

func TestDecoder_EmptySlices(t *testing.T) {
	type TestStruct struct {
		SI []int     `query:"si"`
		SU []uint32  `query:"su"`
		SF []float64 `query:"sf"`
		SB []bool    `query:"sb"`
	}

	ctx := newTestContextWithQuery("")
	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.NoError(t, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.False(t, changed)
	assert.NoError(t, err)
	assert.Len(t, p.SI, 0)
	assert.Len(t, p.SU, 0)
	assert.Len(t, p.SF, 0)
	assert.Len(t, p.SB, 0)
}

func TestDecoder_Struct(t *testing.T) {
	type TestStruct0 struct {
		A string `json:"a" query:"a"`
		B string `json:"b" query:"b"`
	}

	type TestStruct struct {
		X TestStruct0 `query:"x"`
		A string      `json:"a" query:"a"`
		B string      `json:"b" query:"b"`
		C string
	}
	ctx := newTestContextWithQuery(`x=%7B%22a%22%3A%22hello%22%2C%20%22b%22%3A%22world%22%7D&b=test&C=defaulttag`)
	c := &DecodeConfig{}
	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), c)
	assert.Nil(t, err, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.True(t, changed)
	assert.NoError(t, err)
	assert.Equal(t, "hello", p.X.A)
	assert.Equal(t, "world", p.X.B)
	assert.Empty(t, p.A)
	assert.Equal(t, "test", p.B)
	assert.Equal(t, "", p.C)
}

// CustomInt implements UnmarshalParam
type CustomInt int

func (c *CustomInt) UnmarshalParam(param string) error {
	// Custom parsing: multiply by 10
	var val int
	_, err := fmt.Sscanf(param, "%d", &val)
	if err != nil {
		return err
	}
	*c = CustomInt(val * 10)
	return nil
}

// CustomString implements encoding.TextUnmarshaler
type CustomString string

func (c *CustomString) UnmarshalText(text []byte) error {
	// Custom parsing: add prefix
	*c = CustomString("prefix_" + string(text))
	return nil
}

func TestDecoder_UnmarshalParam(t *testing.T) {
	var ci CustomInt
	assert.True(t, reflect.PointerTo(reflect.TypeOf(ci)).Implements(paramUnmarshalerType), "CustomInt should implement UnmarshalParam")

	type TestStruct struct {
		A CustomInt   `path:"a"`
		B []CustomInt `query:"b"`
	}

	req := httptest.NewRequest("GET", "http://example.com/?b=1&b=2", nil)
	httpCtx := NewHTTPRequestContext(req)
	ctx := &testContextWrapper{RequestContext: httpCtx, pathParams: map[string]string{"a": "5"}}

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.NoError(t, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.True(t, changed)
	assert.NoError(t, err)
	assert.Equal(t, CustomInt(50), p.A) // 5 * 10
	assert.Len(t, p.B, 2)
	assert.Equal(t, CustomInt(10), p.B[0]) // 1 * 10
	assert.Equal(t, CustomInt(20), p.B[1]) // 2 * 10
}

func TestDecoder_TextUnmarshaler(t *testing.T) {
	type TestStruct struct {
		A CustomString   `path:"a"`
		B []CustomString `query:"b"`
	}

	req := httptest.NewRequest("GET", "http://example.com/?b=world&b=test", nil)
	httpCtx := NewHTTPRequestContext(req)
	ctx := &testContextWrapper{RequestContext: httpCtx, pathParams: map[string]string{"a": "hello"}}

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.NoError(t, err)

	p := &TestStruct{}
	changed, err := dec.Decode(ctx, p)
	assert.True(t, changed)
	assert.NoError(t, err)
	assert.Equal(t, CustomString("prefix_hello"), p.A)
	assert.Len(t, p.B, 2)
	assert.Equal(t, CustomString("prefix_world"), p.B[0])
	assert.Equal(t, CustomString("prefix_test"), p.B[1])
}

// benchmarkMockContext is a fake mock implementation of RequestContext for benchmarks
// It embeds RequestContext without initializing it and sets values directly by key
type benchmarkMockContext struct {
	RequestContext
	PathValue string
	// Query values stored as individual fields to match the test requirements
	QueryTag   string
	QueryCount string
	QueryTags  []string
	FormValue  string
}

func (m *benchmarkMockContext) GetPathValue(key string, result *GetResult) {
	result.Reset()
	if m.PathValue != "" {
		result.SetStr(m.PathValue)
	}
}

func (m *benchmarkMockContext) GetQuery(key string, result *GetResult) {
	result.Reset()
	switch key {
	case "tag":
		if m.QueryTag != "" {
			result.SetStr(m.QueryTag)
		}
	case "count":
		if m.QueryCount != "" {
			result.SetStr(m.QueryCount)
		}
	case "tags":
		if len(m.QueryTags) > 0 {
			result.SetStr(m.QueryTags...)
		}
	}
}

func (m *benchmarkMockContext) GetPostForm(key string, result *GetResult) {
	result.Reset()
	if m.FormValue != "" {
		result.SetStr(m.FormValue)
	}
}

func BenchmarkDecode(b *testing.B) {
	type BenchStruct struct {
		Path  string `path:"id"`
		Form  string `form:"name"`
		Query string `query:"tag"`
		Count int    `query:"count"`
		Tags  []int  `query:"tags"`
	}

	typ := reflect.TypeOf((*BenchStruct)(nil))
	dec, _ := NewDecoder(typ, &DecodeConfig{})

	// Use fake mock instead of httpRequestContext
	ctx := &benchmarkMockContext{
		PathValue:  "123",
		QueryTag:   "test",
		QueryCount: "42",
		QueryTags:  []string{"1", "2", "3"},
		FormValue:  "alice",
	}

	b.ResetTimer()
	p := &BenchStruct{}
	for i := 0; i < b.N; i++ {
		*p = BenchStruct{} // reset to avoid alloc
		_, err := dec.Decode(ctx, p)
		if err != nil {
			b.Fatal(err)
		}
	}
}
