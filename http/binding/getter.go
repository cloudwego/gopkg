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

type getter func(req RequestContext, key string, result *GetResult)

var tag2getter = map[string]getter{
	pathTag:   path,
	formTag:   form,
	queryTag:  query,
	cookieTag: cookie,
	headerTag: header,
}

func path(req RequestContext, key string, result *GetResult) {
	req.GetPathValue(key, result)
}

func form(req RequestContext, key string, result *GetResult) {
	req.GetPostForm(key, result)
	if len(result.Values()) == 0 {
		req.GetQuery(key, result)
	}
}

func query(req RequestContext, key string, result *GetResult) {
	req.GetQuery(key, result)
}

func cookie(req RequestContext, key string, result *GetResult) {
	req.GetCookie(key, result)
}

func header(req RequestContext, key string, result *GetResult) {
	req.GetHeader(key, result)
}
