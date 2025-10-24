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
	"reflect"
	"strings"
)

const (
	pathTag     = "path"
	formTag     = "form"
	queryTag    = "query"
	cookieTag   = "cookie"
	headerTag   = "header"
	fileNameTag = "file_name"
)

var defaultTags = []string{pathTag, formTag, queryTag, cookieTag, headerTag}

type tagInfo struct {
	Tag    string
	Name   string
	Getter getter

	// Currently unused, reserved for future extensions
	Options []string
}

func newTagInfo(tag, name string) *tagInfo {
	ti := &tagInfo{Tag: tag, Name: name}
	ti.Getter = tag2getter[ti.Tag]
	return ti
}

func (ti *tagInfo) Parse(tagvalue string) {
	tagname, opts, _ := strings.Cut(tagvalue, ",")
	if len(tagname) > 0 {
		ti.Name = tagname
	}
	ti.Options = ti.Options[:0]
	for opts != "" {
		o := ""
		o, opts, _ = strings.Cut(opts, ",")
		ti.Options = append(ti.Options, o)
	}
}

func (ti *tagInfo) Skip() bool { return ti.Name == "-" }

func lookupFieldTags(field reflect.StructField, config *DecodeConfig) []*tagInfo {
	var tagInfos []*tagInfo
	tags := config.Tags
	if len(tags) == 0 {
		tags = defaultTags
	}
	for _, tag := range tags {
		tagv, ok := field.Tag.Lookup(tag)
		if !ok {
			continue
		}
		ti := newTagInfo(tag, field.Name)
		ti.Parse(tagv)
		if ti.Skip() {
			continue
		}
		tagInfos = append(tagInfos, ti)
	}
	return tagInfos
}
