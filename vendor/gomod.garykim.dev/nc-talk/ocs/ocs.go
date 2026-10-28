// Copyright (c) 2020 Gary Kim <gary@garykim.dev>, All Rights Reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ocs

import (
	"fmt"
	"net/http"
	"strings"
)

// StatusOK is the status code the OCS API returns when a request succeeded.
const StatusOK = 100

const maxStatusDetailBody = 300

type ocs struct {
	OCSMeta OCSMeta `json:"meta"`
}

type OCSMeta struct {
	Status     string `json:"status"`
	StatusCode int    `json:"statuscode"`
	Message    string `json:"message"`
}

// StatusDetail describes an unexpected HTTP response so that it can be
// diagnosed. Nextcloud sends the actual reason in the response body, so it is
// included (truncated) alongside the status.
func StatusDetail(statusCode int, body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) > maxStatusDetailBody {
		trimmed = trimmed[:maxStatusDetailBody] + "..."
	}
	text := http.StatusText(statusCode)
	if trimmed == "" {
		return fmt.Sprintf("%d %s", statusCode, text)
	}
	return fmt.Sprintf("%d %s: %s", statusCode, text, trimmed)
}

// OCSStatusDetail describes a response the OCS API reported as unsuccessful
// even though the HTTP request itself may have succeeded.
func OCSStatusDetail(statusCode int, message string) string {
	if message == "" {
		return fmt.Sprintf("OCS status %d", statusCode)
	}
	return fmt.Sprintf("OCS status %d: %s", statusCode, message)
}
