package akips

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	timestampLayout = "2006-01-02 15:04"
	errPrefix       = "ERROR: "
)

var ErrFields = errors.New("akips: incorrect number of fields")

func splitCSV(s string) ([]string, error) {
	ret := make([]string, 0)
	for i := 0; i < len(s); {
		if s[i] == '"' {
			var token strings.Builder
			i++
			for {
				start := i
				for ; i < len(s) && s[i] != '"'; i++ {
				}
				token.WriteString(s[start:i])
				if i < len(s) {
					i++
				}
				if i < len(s) && s[i] == '"' {
					i++
					token.WriteByte('"')
				} else {
					break
				}
			}
			ret = append(ret, token.String())
			if i < len(s) {
				if s[i] != ',' {
					return ret, fmt.Errorf("akips: unexpected character at position %d: '%c'", i, s[i])
				}
				i++
				if i == len(s) {
					ret = append(ret, "")
				}
			}
		} else {
			start := i
			for ; i < len(s) && s[i] != ','; i++ {
			}
			ret = append(ret, s[start:i])
			if i < len(s) {
				i++
				if i == len(s) {
					ret = append(ret, "")
				}
			}
		}
	}
	return ret, nil
}

func isError(s string) (string, bool) {
	if strings.HasPrefix(s, errPrefix) {
		return s[len(errPrefix):], true
	}
	return "", false
}

// GenericResponseEntry holds parent, child, attribute and a list of string values.
type GenericResponseEntry struct {
	Parent    string   `json:"parent,omitempty"`
	Child     string   `json:"child,omitempty"`
	Attribute string   `json:"attr,omitempty"`
	Values    []string `json:"val,omitempty"`
}

// GenericResponse is a slice of GenericResponseEntry, used for time_series and table modes.
type GenericResponse []*GenericResponseEntry

func (p *GenericResponse) ParseResponse(rd io.Reader) error {
	res := GenericResponse{}

	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		if e, ok := isError(sc.Text()); ok {
			return fmt.Errorf("akips: %s", e)
		}

		var entry GenericResponseEntry
		kv := strings.SplitN(sc.Text(), "=", 2)
		pca := strings.Fields(kv[0])
		if len(pca) != 0 {
			entry.Parent = pca[0]
		}
		if len(pca) > 1 {
			entry.Child = pca[1]
		}
		if len(pca) > 2 {
			entry.Attribute = pca[2]
		}
		if len(kv) > 1 {
			v, err := splitCSV(strings.TrimSpace(kv[1]))
			if err != nil {
				return ErrFields
			}
			entry.Values = v
		}
		res = append(res, &entry)
	}

	if err := sc.Err(); err != nil {
		return err
	}
	*p = res
	return nil
}

// CSVResponse is used for the `get` command — rows of comma-separated values.
type CSVResponse [][]string

func (c *CSVResponse) ParseResponse(rd io.Reader) error {
	res := [][]string{}
	sc := bufio.NewScanner(rd)

	for sc.Scan() {
		if e, ok := isError(sc.Text()); ok {
			return fmt.Errorf("akips: %s", e)
		}

		v, err := splitCSV(sc.Text())
		if err != nil {
			return ErrFields
		}
		res = append(res, v)
	}

	if err := sc.Err(); err != nil {
		return err
	}
	*c = res
	return nil
}

// TestResponse is used for health-check calls — it only surfaces ERROR lines.
type TestResponse struct{}

func (t TestResponse) ParseResponse(rd io.Reader) error {
	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		if e, ok := isError(sc.Text()); ok {
			return fmt.Errorf("akips: %s", e)
		}
	}
	return sc.Err()
}

// TimeSeriesResponse holds a header row of timestamps followed by data rows.
// Each row maps to a (parent, child, childDesc, attribute) tuple plus values.
type TimeSeriesResponseEntry struct {
	Parent           string  `json:"parent,omitempty"`
	Child            string  `json:"child,omitempty"`
	ChildDescription string  `json:"childDesc,omitempty"`
	Attribute        string  `json:"attr,omitempty"`
	Values           []int64 `json:"val"`
}

type TimeSeriesResponse struct {
	Timestamp []time.Time               `json:"ts"`
	Entries   []*TimeSeriesResponseEntry `json:"entries"`
}

func (t *TimeSeriesResponse) ParseResponse(rd io.Reader) error {
	res := TimeSeriesResponse{
		Entries: make([]*TimeSeriesResponseEntry, 0),
	}

	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		if e, ok := isError(sc.Text()); ok {
			return fmt.Errorf("akips: %s", e)
		}
		rec, err := splitCSV(sc.Text())
		if err != nil {
			return err
		}
		if len(rec) < 4 {
			return ErrFields
		}

		if res.Timestamp == nil {
			res.Timestamp = make([]time.Time, len(rec)-4)
			for i, v := range rec[4:] {
				ts, err := time.Parse(timestampLayout, v)
				if err != nil {
					return err
				}
				res.Timestamp[i] = ts.UTC()
			}
		} else {
			ivalues := make([]int64, len(rec)-4)
			for i, v := range rec[4:] {
				if v != "" {
					iv, err := strconv.ParseInt(v, 10, 64)
					if err != nil {
						return err
					}
					ivalues[i] = iv
				}
			}
			entry := TimeSeriesResponseEntry{
				Parent:           rec[0],
				Child:            rec[1],
				ChildDescription: rec[2],
				Attribute:        rec[3],
				Values:           ivalues,
			}
			res.Entries = append(res.Entries, &entry)
		}
	}

	if err := sc.Err(); err != nil {
		return err
	}
	*t = res
	return nil
}
