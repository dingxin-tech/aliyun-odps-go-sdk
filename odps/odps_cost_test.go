// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package odps

import (
	"strings"
	"testing"
)

// TestEstimateSQLCost_PlaceholderReturnsError documents the placeholder contract:
// the method exists with the expected signature but is intentionally not yet
// implemented. Replace this test when the real REST plumbing lands.
func TestEstimateSQLCost_PlaceholderReturnsError(t *testing.T) {
	o := &Odps{}
	cost, err := o.EstimateSQLCost("select 1;", nil)
	if err == nil {
		t.Fatalf("expected error from placeholder implementation, got nil with cost=%v", cost)
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' marker in placeholder error, got: %v", err)
	}
	if cost != nil {
		t.Errorf("expected nil SQLCost from placeholder, got %v", cost)
	}
}
