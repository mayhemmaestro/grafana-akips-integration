package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/mayhemmaestro/grafana-akips-integration/pkg/models"
)

func newTestDatasource(serverURL string, client *http.Client) *Datasource {
	return &Datasource{
		Client: client,
		Settings: &models.PluginSettings{
			URL:      serverURL,
			Username: "admin",
			Secrets:  &models.SecretPluginSettings{Password: "secret"},
		},
	}
}

func newDataQuery(t *testing.T, model queryModel) backend.DataQuery {
	t.Helper()
	b, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("marshal queryModel: %v", err)
	}
	return backend.DataQuery{
		RefID:     "A",
		JSON:      b,
		TimeRange: backend.TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
	}
}

func TestCredentialsAreSentAsQueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") != "admin" {
			t.Errorf("expected username=admin, got %q", r.URL.Query().Get("username"))
		}
		if r.URL.Query().Get("password") != "secret" {
			t.Errorf("expected password=secret, got %q", r.URL.Query().Get("password"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ds := newTestDatasource(server.URL, server.Client())
	q := newDataQuery(t, queryModel{QueryText: "mget * * sysName", APIEndpoint: "api-db"})
	ds.QueryData(context.Background(), &backend.QueryDataRequest{Queries: []backend.DataQuery{q}})
}

func TestInterpolateVariables(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)

	qctx := &queryContext{
		dq: &backend.DataQuery{
			Interval:  60 * time.Second,
			TimeRange: backend.TimeRange{From: from, To: to},
		},
		model: &queryModel{
			QueryText: "series $__timeInterval $__timeFrom $__timeTo $__device",
			Device:    "router01",
		},
	}
	result := qctx.interpolateVariables()
	expected := "series 60 1704067200 1704070800 router01"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestQueryTimeSeries(t *testing.T) {
	body := "router01 sys sysUpTime = 100,200,300\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	ds := newTestDatasource(server.URL, server.Client())
	q := newDataQuery(t, queryModel{QueryText: "series sysUpTime", QueryType: "time_series"})

	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{Queries: []backend.DataQuery{q}})
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	res := resp.Responses["A"]
	if res.Error != nil {
		t.Fatalf("response error: %v", res.Error)
	}
	if len(res.Frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(res.Frames))
	}
	// Frame has Timestamp + value field
	if len(res.Frames[0].Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(res.Frames[0].Fields))
	}
	if res.Frames[0].Fields[1].Len() != 3 {
		t.Errorf("expected 3 datapoints, got %d", res.Frames[0].Fields[1].Len())
	}
}

func TestQueryTable(t *testing.T) {
	body := "router01 sys sysName = Hostname01\nrouter02 sys sysName = Hostname02\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	ds := newTestDatasource(server.URL, server.Client())
	q := newDataQuery(t, queryModel{QueryText: "mget * sys sysName", QueryType: "table"})

	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{Queries: []backend.DataQuery{q}})
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	res := resp.Responses["A"]
	if res.Error != nil {
		t.Fatalf("response error: %v", res.Error)
	}
	if len(res.Frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(res.Frames))
	}
	frame := res.Frames[0]
	// Columns: Parent, Child, Attribute, Value #0
	if len(frame.Fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(frame.Fields))
	}
	if frame.Fields[0].Len() != 2 {
		t.Errorf("expected 2 rows, got %d", frame.Fields[0].Len())
	}
}

func TestQueryCSV(t *testing.T) {
	body := "10,20,30\n40,50,60\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	ds := newTestDatasource(server.URL, server.Client())
	q := newDataQuery(t, queryModel{QueryText: "get router01 sys ip4addr", QueryType: "csv"})

	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{Queries: []backend.DataQuery{q}})
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	res := resp.Responses["A"]
	if res.Error != nil {
		t.Fatalf("response error: %v", res.Error)
	}
	if len(res.Frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(res.Frames))
	}
	// 3 columns: Value #0, Value #1, Value #2
	if len(res.Frames[0].Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(res.Frames[0].Fields))
	}
}

func TestCheckHealthMissingConfig(t *testing.T) {
	ds := &Datasource{
		Client: http.DefaultClient,
		Settings: &models.PluginSettings{
			Secrets: &models.SecretPluginSettings{},
		},
	}
	result, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != backend.HealthStatusError {
		t.Errorf("expected error status, got %v", result.Status)
	}
}

func TestCheckHealthSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") != "admin" || r.URL.Query().Get("password") != "secret" {
			t.Errorf("expected username=admin&password=secret, got %s/%s", r.URL.Query().Get("username"), r.URL.Query().Get("password"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("router01 sys sysName\n"))
	}))
	defer server.Close()

	ds := newTestDatasource(server.URL, server.Client())
	result, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != backend.HealthStatusOk {
		t.Errorf("expected ok, got %v: %s", result.Status, result.Message)
	}
}
