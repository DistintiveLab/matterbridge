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
       "bytes"
       "encoding/json"
       "fmt"
)

// Capabilities describes the response from the capabilities request
type Capabilities struct {
	ocs
	Data struct {
		Capabilities struct {
			SpreedCapabilities SpreedCapabilities `json:"spreed"`
		} `json:"capabilities"`
	} `json:"data"`
}

// UnmarshalJSON accepts both the legacy object form of ocs.data and the
// array form returned by Nextcloud 33+.
func (c *Capabilities) UnmarshalJSON(data []byte) error {
       type dataShape struct {
               Capabilities struct {
                       SpreedCapabilities SpreedCapabilities `json:"spreed"`
               } `json:"capabilities"`
       }

       var raw struct {
               OCSMeta ocsMeta         `json:"meta"`
               Data    json.RawMessage `json:"data"`
       }
       if err := json.Unmarshal(data, &raw); err != nil {
               return err
       }
       c.ocs.OCSMeta = raw.OCSMeta

       trimmed := bytes.TrimSpace(raw.Data)
       if len(trimmed) == 0 {
               return nil
       }

       switch trimmed[0] {
       case '{':
               var d dataShape
               if err := json.Unmarshal(trimmed, &d); err != nil {
                       return err
               }
               c.Data = d
       case '[':
               var arr []dataShape
               if err := json.Unmarshal(trimmed, &arr); err != nil {
                       return err
               }
               for _, d := range arr {
                       if len(d.Capabilities.SpreedCapabilities.Features) > 0 {
                               c.Data = d
                               break
                       }
               }
               if c.Data.Capabilities.SpreedCapabilities.Features == nil && len(arr) > 0 {
                       c.Data = arr[0]
               }
       default:
               return fmt.Errorf("capabilities data is neither an object nor an array")
       }
       return nil
}

// SpreedCapabilities describes the Nextcloud Talk capabilities response
type SpreedCapabilities struct {
	Features []string `json:"features"`
	Config   struct {
		Attachments struct {
			Allowed bool   `json:"allowed"`
			Folder  string `json:"folder"`
		} `json:"attachments"`
		Chat struct {
			MaxLength   int `json:"max-length"`
			ReadPrivacy int `json:"read-privacy"`
		} `json:"chat"`
		Conversations struct {
			CanCreate bool `json:"can-create"`
		} `json:"conversations"`
		Previews struct {
			MaxGifSize int `json:"max-gif-size"`
		} `json:"previews"`
	} `json:"config"`
}
