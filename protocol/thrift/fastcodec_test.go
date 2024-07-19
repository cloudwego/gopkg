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

	"github.com/stretchr/testify/require"
)

func TestFastMarshal(t *testing.T) {
	req1, req2 := NewApplicationException(1, "hello"), NewApplicationException(0, "")
	buf := FastMarshal(req1)
	err := FastUnmarshal(buf, req2)
	require.NoError(t, err)
	require.Equal(t, req1.t, req2.t)
	require.Equal(t, req1.m, req2.m)
}

func TestMarshalFastMsg(t *testing.T) {
	// CALL and REPLY

	req := NewApplicationException(1, "hello")
	b, err := MarshalFastMsg("Echo", CALL, 1, req)
	require.NoError(t, err)

	resp := NewApplicationException(0, "")
	method, seq, err := UnmarshalFastMsg(b, resp)
	require.NoError(t, err)
	require.Equal(t, "Echo", method)
	require.Equal(t, int32(1), seq)
	require.Equal(t, req.t, resp.t)
	require.Equal(t, req.m, resp.m)

	// EXCEPTION

	ex := NewApplicationException(WRONG_METHOD_NAME, "Ex!")
	b, err = MarshalFastMsg("ExMethod", EXCEPTION, 2, ex)
	require.NoError(t, err)
	method, seq, err = UnmarshalFastMsg(b, nil)
	require.NotNil(t, err)
	require.Equal(t, "ExMethod", method)
	require.Equal(t, int32(2), seq)
	e, ok := err.(*ApplicationException)
	require.True(t, ok)
	require.True(t, e.TypeID() == ex.TypeID() && e.Error() == ex.Error())
}
