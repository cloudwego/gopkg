/*
 * Copyright 2024 CloudWeGo Authors
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

package thrift

import (
	"testing"

	"github.com/cloudwego/gopkg/internal/assert"
)

func TestFastMarshal(t *testing.T) {
	req1, req2 := NewApplicationException(1, "hello"), NewApplicationException(0, "")
	buf := FastMarshal(req1)
	err := FastUnmarshal(buf, req2)
	assert.Nil(t, err)
	assert.Equal(t, req1.t, req2.t)
	assert.Equal(t, req1.m, req2.m)
}

func TestMarshalFastMsg(t *testing.T) {
	// CALL and REPLY

	req := NewApplicationException(1, "hello")
	b, err := MarshalFastMsg("Echo", CALL, 1, req)
	assert.Nil(t, err)

	resp := NewApplicationException(0, "")
	method, seq, err := UnmarshalFastMsg(b, resp)
	assert.Nil(t, err)
	assert.Equal(t, "Echo", method)
	assert.Equal(t, int32(1), seq)
	assert.Equal(t, req.t, resp.t)
	assert.Equal(t, req.m, resp.m)

	// EXCEPTION

	ex := NewApplicationException(WRONG_METHOD_NAME, "Ex!")
	b, err = MarshalFastMsg("ExMethod", EXCEPTION, 2, ex)
	assert.Nil(t, err)
	method, seq, err = UnmarshalFastMsg(b, nil)
	assert.True(t, err != nil)
	assert.Equal(t, "ExMethod", method)
	assert.Equal(t, int32(2), seq)
	e, ok := err.(*ApplicationException)
	assert.True(t, ok)
	assert.True(t, e.TypeID() == ex.TypeID() && e.Error() == ex.Error())
}
