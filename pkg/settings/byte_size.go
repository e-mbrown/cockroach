// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package settings

import "github.com/cockroachdb/cockroach/pkg/util/humanizeutil"

// ByteSizeSetting is the interface of a setting variable that will be
// updated automatically when the corresponding cluster-wide setting
// of type "bytesize" is updated.
type ByteSizeSetting struct {
	IntSetting
}

var _ Setting = &ByteSizeSetting{}

// Typ returns the short (1 char) string denoting the type of setting.
func (*ByteSizeSetting) Typ() string {
	return "z"
}

func (b *ByteSizeSetting) String() string {
	return humanizeutil.IBytes(b.Get())
}

// RegisterByteSizeSetting defines a new setting with type bytesize.
func RegisterByteSizeSetting(key, desc string, defaultValue int64) *ByteSizeSetting {
	setting := &ByteSizeSetting{IntSetting{defaultValue: defaultValue}}
	register(key, desc, setting)
	return setting
}

// TestingSetByteSize returns a mock bytesize setting for testing. See
// TestingSetBool for more details.
func TestingSetByteSize(s **ByteSizeSetting, v int64) func() {
	saved := *s
	*s = &ByteSizeSetting{IntSetting{v: v}}
	return func() {
		*s = saved
	}
}
