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

// Code generated by thriftgo (0.3.15). DO NOT EDIT.

package base

import (
	"fmt"
)

type Base struct {
	LogID  string            `thrift:"LogID,1" json:"LogID"`
	Caller string            `thrift:"Caller,2" json:"Caller"`
	Addr   string            `thrift:"Addr,3" json:"Addr"`
	Extra  map[string]string `thrift:"Extra,6,optional" json:"Extra,omitempty"`
}

func NewBase() *Base {
	return &Base{

		LogID:  "",
		Caller: "",
		Addr:   "",
	}
}

func (p *Base) InitDefault() {
	p.LogID = ""
	p.Caller = ""
	p.Addr = ""
}

func (p *Base) GetLogID() (v string) {
	return p.LogID
}

func (p *Base) GetCaller() (v string) {
	return p.Caller
}

func (p *Base) GetAddr() (v string) {
	return p.Addr
}

var Base_Extra_DEFAULT map[string]string

func (p *Base) GetExtra() (v map[string]string) {
	if !p.IsSetExtra() {
		return Base_Extra_DEFAULT
	}
	return p.Extra
}
func (p *Base) SetLogID(val string) {
	p.LogID = val
}
func (p *Base) SetCaller(val string) {
	p.Caller = val
}
func (p *Base) SetAddr(val string) {
	p.Addr = val
}
func (p *Base) SetExtra(val map[string]string) {
	p.Extra = val
}

func (p *Base) IsSetExtra() bool {
	return p.Extra != nil
}

func (p *Base) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Base(%+v)", *p)
}

var fieldIDToName_Base = map[int16]string{
	1: "LogID",
	2: "Caller",
	3: "Addr",
	6: "Extra",
}

type BaseResp struct {
	StatusMessage string            `thrift:"StatusMessage,1" json:"StatusMessage"`
	StatusCode    int32             `thrift:"StatusCode,2" json:"StatusCode"`
	Extra         map[string]string `thrift:"Extra,3,optional" json:"Extra,omitempty"`
}

func NewBaseResp() *BaseResp {
	return &BaseResp{

		StatusMessage: "",
		StatusCode:    0,
	}
}

func (p *BaseResp) InitDefault() {
	p.StatusMessage = ""
	p.StatusCode = 0
}

func (p *BaseResp) GetStatusMessage() (v string) {
	return p.StatusMessage
}

func (p *BaseResp) GetStatusCode() (v int32) {
	return p.StatusCode
}

var BaseResp_Extra_DEFAULT map[string]string

func (p *BaseResp) GetExtra() (v map[string]string) {
	if !p.IsSetExtra() {
		return BaseResp_Extra_DEFAULT
	}
	return p.Extra
}
func (p *BaseResp) SetStatusMessage(val string) {
	p.StatusMessage = val
}
func (p *BaseResp) SetStatusCode(val int32) {
	p.StatusCode = val
}
func (p *BaseResp) SetExtra(val map[string]string) {
	p.Extra = val
}

func (p *BaseResp) IsSetExtra() bool {
	return p.Extra != nil
}

func (p *BaseResp) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaseResp(%+v)", *p)
}

var fieldIDToName_BaseResp = map[int16]string{
	1: "StatusMessage",
	2: "StatusCode",
	3: "Extra",
}
