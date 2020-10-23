// Copyright (C) 2020 Google Inc.
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

package utils

import (
	"strings"

	"github.com/google/gapid/core/os/device"
)

// Format a counter's unit. Translate from the original numerator units and
// denominator units to a single string value.
func FormatUnit(numerators, denominators []device.GpuCounterDescriptor_MeasureUnit) string {
	var ns, ds string
	for _, n := range numerators {
		if n != device.GpuCounterDescriptor_NONE {
			if len(ns) > 0 {
				ns += "*"
			}
			ns += strings.ToLower(n.String())
		}
	}
	for _, d := range denominators {
		if d != device.GpuCounterDescriptor_NONE {
			if len(ds) > 0 {
				ds += "*"
			}
			ds += strings.ToLower(d.String())
		}
	}
	if len(ds) == 0 {
		return ns
	} else {
		if strings.Contains(ns, "*") {
			ns = "(" + ns + ")"
		}
		if strings.Contains(ds, "*") {
			ds = "(" + ds + ")"
		}
		return ns + "/" + ds
	}
}
