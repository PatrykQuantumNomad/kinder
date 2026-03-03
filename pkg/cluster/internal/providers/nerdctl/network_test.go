/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nerdctl

import (
	"fmt"
	"testing"
)

func Test_generateULASubnetFromName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		attempt int32
		subnet  string
	}{
		{
			name:   "kind",
			subnet: "fc00:4d57:1afd:1f1b::/64",
		},
		{
			name:    "foo",
			attempt: 1,
			subnet:  "fc00:3b86:d0fc:521c::/64",
		},
		{
			name:    "foo",
			attempt: 2,
			subnet:  "fc00:2774:110d:111a::/64",
		},
		{
			name:   "kind2",
			subnet: "fc00:724:8e67:eb33::/64",
		},
		{
			name:   "kin",
			subnet: "fc00:34fb:893a:fe4a::/64",
		},
		{
			name:   "mysupernetwork",
			subnet: "fc00:1567:8728:937b::/64",
		},
	}
	for _, tc := range cases {
		tc := tc // capture variable
		t.Run(fmt.Sprintf("%s,%d", tc.name, tc.attempt), func(t *testing.T) {
			t.Parallel()
			subnet := generateULASubnetFromName(tc.name, tc.attempt)
			if subnet != tc.subnet {
				t.Errorf("Wrong subnet from %v: expected %v, received %v", tc.name, tc.subnet, subnet)
			}
		})
	}
}
