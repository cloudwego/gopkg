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

package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBase(t *testing.T) {
	var err error

	p := NewBase()
	p.InitDefault() // for code coverage ...
	p.LogID = "1"
	p.Caller = "2"
	p.Addr = "3"
	p.Client = "4"
	t.Log(p.String())

	sz := p.BLength()
	b := make([]byte, sz)
	n := p.FastWrite(b)
	require.Equal(t, sz, n)

	p2 := NewBase()
	n, err = p2.FastRead(b)
	require.Equal(t, len(b), n)
	require.NoError(t, err)
	require.Equal(t, p, p2)

	// optional fields

	p.TrafficEnv = NewTrafficEnv()
	p.TrafficEnv.InitDefault() // for code coverage ...
	p.Extra = map[string]string{"5": "6"}
	t.Log(p.String())

	sz = p.BLength()
	b = make([]byte, sz)
	n = p.FastWrite(b)
	require.Equal(t, sz, n)

	p2 = NewBase()
	n, err = p2.FastRead(b)
	require.Equal(t, len(b), n)
	require.NoError(t, err)
	require.Equal(t, p, p2)
}

func TestBaseResp(t *testing.T) {
	var err error

	p := NewBaseResp()
	p.InitDefault() // for code coverage ...
	p.StatusMessage = "msg"
	p.StatusCode = 200
	p.Extra = map[string]string{"k": "v"}
	t.Log(p.String())

	sz := p.BLength()
	b := make([]byte, sz)
	n := p.FastWrite(b)
	require.Equal(t, sz, n)

	p2 := NewBaseResp()
	n, err = p2.FastRead(b)
	require.Equal(t, len(b), n)
	require.NoError(t, err)
	require.Equal(t, p, p2)
}
