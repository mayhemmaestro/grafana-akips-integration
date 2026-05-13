package akips

import (
	"strings"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{`"a,b",c`, []string{"a,b", "c"}},
		{`"a""b",c`, []string{`a"b`, "c"}},
		{"a,,c", []string{"a", "", "c"}},
		{"a,b,", []string{"a", "b", ""}},
	}
	for _, tc := range cases {
		got, err := splitCSV(tc.input)
		if err != nil {
			t.Errorf("splitCSV(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("splitCSV(%q): got %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitCSV(%q)[%d]: got %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestGenericResponseParse(t *testing.T) {
	body := `router01 sys sysName = Hostname01
router01 eth0 ifDescr = Interface0
`
	var r GenericResponse
	if err := r.ParseResponse(strings.NewReader(body)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(r))
	}
	if r[0].Parent != "router01" || r[0].Child != "sys" || r[0].Attribute != "sysName" {
		t.Errorf("first entry wrong: %+v", r[0])
	}
	if len(r[0].Values) != 1 || r[0].Values[0] != "Hostname01" {
		t.Errorf("first entry values wrong: %v", r[0].Values)
	}
}

func TestGenericResponseError(t *testing.T) {
	body := "ERROR: something went wrong\n"
	var r GenericResponse
	err := r.ParseResponse(strings.NewReader(body))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCSVResponseParse(t *testing.T) {
	body := "val1,val2,val3\nval4,val5,val6\n"
	var r CSVResponse
	if err := r.ParseResponse(strings.NewReader(body)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(r))
	}
	if r[0][1] != "val2" {
		t.Errorf("expected val2, got %q", r[0][1])
	}
}

func TestCSVResponseError(t *testing.T) {
	body := "ERROR: permission denied\n"
	var r CSVResponse
	if err := r.ParseResponse(strings.NewReader(body)); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTestResponseOK(t *testing.T) {
	body := "some output\nmore output\n"
	var r TestResponse
	if err := r.ParseResponse(strings.NewReader(body)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTestResponseError(t *testing.T) {
	body := "ERROR: auth failed\n"
	var r TestResponse
	if err := r.ParseResponse(strings.NewReader(body)); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTimeSeriesResponseParse(t *testing.T) {
	body := "router01,eth0,Interface 0,ifInOctets,2024-01-01 00:00,2024-01-01 00:01\nrouter01,eth0,Interface 0,ifInOctets,100,200\n"
	var r TimeSeriesResponse
	if err := r.ParseResponse(strings.NewReader(body)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Timestamp) != 2 {
		t.Fatalf("expected 2 timestamps, got %d", len(r.Timestamp))
	}
	if len(r.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r.Entries))
	}
	if r.Entries[0].Values[0] != 100 || r.Entries[0].Values[1] != 200 {
		t.Errorf("unexpected values: %v", r.Entries[0].Values)
	}
}
