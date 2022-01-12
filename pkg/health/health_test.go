/*
Copyright 2019, 2021 The Multi-Cluster App Dispatcher Authors.

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
package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthProbe(t *testing.T) {
	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := Handler{}
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned unexpected status: %v", status)
	}

	expected := "ok"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v expected %v",
			rr.Body.String(), expected)
	}
}
