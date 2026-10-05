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
	"reflect"
)

var (
	fileBindingType = reflect.TypeOf((*multipart.FileHeader)(nil)).Elem()
)

// getFileNameFromFieldInfo extracts file name from tags (priority: file_name > form > field name)
func getFileNameFromFieldInfo(fi *fieldInfo) string {
	var fileName string
	for _, ti := range fi.tagInfos {
		if ti.Tag == fileNameTag {
			return ti.Name
		}
		if ti.Tag == formTag {
			fileName = ti.Name
		}
	}
	if fileName != "" {
		return fileName
	}
	return fi.fieldName
}

// fileTypeDecoder handles single multipart.FileHeader fields
type fileTypeDecoder struct {
	*fieldInfo
	fileName string
}

func newFileTypeDecoder(fi *fieldInfo) fieldDecoder {
	return &fileTypeDecoder{
		fieldInfo: fi,
		fileName:  getFileNameFromFieldInfo(fi),
	}
}

func (d *fileTypeDecoder) Decode(ctx *decodeCtx, rv reflect.Value) (bool, error) {
	files, err := ctx.GetFormFiles(d.fileName)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		return false, nil
	}
	d.FieldValue(rv).Set(reflect.ValueOf(*files[0]))
	return true, nil
}

// fileTypeSliceDecoder handles slice/array of multipart.FileHeader fields
type fileTypeSliceDecoder struct {
	*fieldInfo
	fileName string
}

func newFileTypeSliceDecoder(fi *fieldInfo) fieldDecoder {
	return &fileTypeSliceDecoder{
		fieldInfo: fi,
		fileName:  getFileNameFromFieldInfo(fi),
	}
}

func (d *fileTypeSliceDecoder) Decode(ctx *decodeCtx, rv reflect.Value) (bool, error) {
	files, err := ctx.GetFormFiles(d.fileName)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		return false, nil
	}

	if ft := d.fieldType; ft.Kind() == reflect.Array && len(files) != ft.Len() {
		return false, fmt.Errorf("the numbers(%d) of file '%s' does not match the length(%d) of %s",
			len(files), d.fileName, ft.Len(), ft.String())
	}

	field := d.FieldValue(rv)
	if field.Kind() == reflect.Slice {
		field.Set(reflect.MakeSlice(field.Type(), len(files), len(files)))
	}
	for i, file := range files {
		v := dereference2lvalue(field.Index(i))
		v.Set(reflect.ValueOf(*file))
	}
	return true, nil
}
