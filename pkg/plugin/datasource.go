package plugin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/converters"
	"github.com/mayhemmaestro/grafana-akips-integration/pkg/akips"
	"github.com/mayhemmaestro/grafana-akips-integration/pkg/models"
)

var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

const minInterval = 60 * time.Second

func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	config, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.SkipTLS},
	}

	return &Datasource{
		Client:   &http.Client{Transport: transport},
		Settings: config,
	}, nil
}

type Datasource struct {
	Client   *http.Client
	Settings *models.PluginSettings
}

func (d *Datasource) Dispose() {}

// QueryData handles multiple queries and returns multiple responses.
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()
	for i := range req.Queries {
		res := d.query(ctx, req.Queries[i])
		response.Responses[req.Queries[i].RefID] = res
	}
	return response, nil
}

type queryModel struct {
	QueryText   string `json:"queryText"`
	APIEndpoint string `json:"apiEndpoint"`
	QueryType   string `json:"queryType"` // "time_series" | "table" | "csv"
	Device      string `json:"device"`
	Child       string `json:"child"`
	Attribute   string `json:"attribute"`
	OmitParents bool   `json:"omitParents"`
}

type queryContext struct {
	dq    *backend.DataQuery
	model *queryModel
}

// interpolateVariables replaces AKIPS-specific template variables in queryText.
func (q *queryContext) interpolateVariables() string {
	replace := func(s, name, val string) string {
		re := regexp.MustCompile(`\$(` + name + `(\W|$)|{` + name + `})`)
		return re.ReplaceAllString(s, val+"$2")
	}

	interval := int64(((q.dq.Interval + minInterval - 1) / minInterval) * minInterval / time.Second)
	from := q.dq.TimeRange.From.Unix()
	to := q.dq.TimeRange.To.Unix()

	attr := q.model.Attribute
	if attr == "" {
		attr = "*"
	}

	vars := [][2]string{
		{"__timeInterval", strconv.FormatInt(interval, 10)},
		{"__timeFrom", strconv.FormatInt(from, 10)},
		{"__timeTo", strconv.FormatInt(to, 10)},
		{"__device", q.model.Device},
		{"__child", q.model.Child},
		{"__attribute", attr},
	}

	qstr := q.model.QueryText
	for _, v := range vars {
		qstr = replace(qstr, v[0], v[1])
	}
	return qstr
}

// mkTimestampField creates a timestamp field evenly spread over the query time range.
func (q *queryContext) mkTimestampField(n int) *data.Field {
	ts := make([]time.Time, n)
	d := n - 1
	if d == 0 {
		d = 1
	}
	dur := q.dq.TimeRange.Duration()
	for i := range ts {
		ts[i] = q.dq.TimeRange.From.Add(dur * time.Duration(i) / time.Duration(d))
	}
	return data.NewField("Timestamp", nil, ts)
}

// doAKIPS executes a single command against the given AKIPS endpoint and returns the raw body.
func (d *Datasource) doAKIPS(ctx context.Context, endpoint, cmd string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", d.Settings.URL, endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	// Set RawQuery after request creation so Go's url.Parse never re-encodes
	// the spaces and quotes that AKIPS requires as literals.
	req.URL.RawQuery = strings.ReplaceAll(
		fmt.Sprintf("username=%s;password=%s;cmds=%s", d.Settings.Username, d.Settings.Secrets.Password, cmd),
		" ", "+",
	)

	backend.Logger.Debug("AKIPS request", "url", req.URL.String())

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to AKIPS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		backend.Logger.Error("AKIPS error response", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("AKIPS returned: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// parseMListResponse extracts the deduplicated column at colIdx from space-separated mlist output.
func parseMListResponse(body []byte, colIdx int) ([]string, error) {
	seen := make(map[string]struct{})
	var values []string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "ERROR: ") {
			return nil, fmt.Errorf("akips: %s", line[len("ERROR: "):])
		}
		fields := strings.Fields(line)
		if colIdx < len(fields) {
			v := fields[colIdx]
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				values = append(values, v)
			}
		}
	}
	return values, nil
}

type labeledItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// parseMGetChildrenResponse parses mget output in the form:
//
//	{device} {child} = {child_attr},{desc}
//
// It returns deduplicated items with the child OID as value and the description as label.
func parseMGetChildrenResponse(body []byte) ([]labeledItem, error) {
	seen := make(map[string]struct{})
	var items []labeledItem
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "ERROR: ") {
			return nil, fmt.Errorf("akips: %s", line[len("ERROR: "):])
		}
		eqIdx := strings.Index(line, " = ")
		if eqIdx < 0 {
			continue
		}
		fields := strings.Fields(line[:eqIdx])
		if len(fields) < 2 {
			continue
		}
		child := fields[1]
		if _, ok := seen[child]; ok {
			continue
		}
		seen[child] = struct{}{}

		// desc is everything after the first comma on the right-hand side
		rhs := strings.TrimSpace(line[eqIdx+3:])
		label := child
		if commaIdx := strings.Index(rhs, ","); commaIdx >= 0 {
			if d := strings.TrimSpace(rhs[commaIdx+1:]); d != "" {
				label = d
			}
		}

		items = append(items, labeledItem{Value: child, Label: label})
	}
	return items, nil
}

func sendResourceJSON(sender backend.CallResourceResponseSender, status int, v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return sender.Send(&backend.CallResourceResponse{
		Status:  status,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    body,
	})
}

// CallResource handles frontend requests for device/child/attribute autocomplete.
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	u, err := url.Parse(req.URL)
	if err != nil {
		return sendResourceJSON(sender, http.StatusBadRequest, map[string]string{"error": "invalid URL"})
	}
	params := u.Query()

	switch req.Path {
	case "devices":
		body, err := d.doAKIPS(ctx, "api-db", "mlist device *")
		if err != nil {
			return sendResourceJSON(sender, http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		values, err := parseMListResponse(body, 0)
		if err != nil {
			return sendResourceJSON(sender, http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		sort.Strings(values)
		if values == nil {
			values = []string{}
		}
		return sendResourceJSON(sender, http.StatusOK, map[string]interface{}{"values": values})

	case "children":
		device := params.Get("device")
		if device == "" {
			device = "*"
		}
		body, err := d.doAKIPS(ctx, "api-db", fmt.Sprintf("mget child %s *", device))
		if err != nil {
			return sendResourceJSON(sender, http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		items, err := parseMGetChildrenResponse(body)
		if err != nil {
			return sendResourceJSON(sender, http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
		if items == nil {
			items = []labeledItem{}
		}
		return sendResourceJSON(sender, http.StatusOK, map[string]interface{}{"items": items})

	case "attributes":
		device := params.Get("device")
		child := params.Get("child")
		if device == "" {
			device = "*"
		}
		if child == "" {
			child = "*"
		}
		body, err := d.doAKIPS(ctx, "api-db", fmt.Sprintf("mlist attribute %s %s *", device, child))
		if err != nil {
			return sendResourceJSON(sender, http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		values, err := parseMListResponse(body, 2)
		if err != nil {
			return sendResourceJSON(sender, http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		sort.Strings(values)
		if values == nil {
			values = []string{}
		}
		return sendResourceJSON(sender, http.StatusOK, map[string]interface{}{"values": values})

	default:
		return sender.Send(&backend.CallResourceResponse{Status: http.StatusNotFound})
	}
}

func (d *Datasource) query(ctx context.Context, dq backend.DataQuery) backend.DataResponse {
	var model queryModel
	if err := json.Unmarshal(dq.JSON, &model); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err))
	}

	qctx := &queryContext{dq: &dq, model: &model}
	interpolated := qctx.interpolateVariables()

	endpoint := model.APIEndpoint
	if endpoint == "" {
		endpoint = "api-db"
	}

	rawBody, err := d.doAKIPS(ctx, endpoint, interpolated)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadGateway, err.Error())
	}

	meta := &data.FrameMeta{ExecutedQueryString: interpolated}
	body := bytes.NewReader(rawBody)

	switch model.QueryType {
	case "csv":
		var r akips.CSVResponse
		if err := r.ParseResponse(body); err != nil {
			return backend.ErrDataResponse(backend.StatusInternal, err.Error())
		}
		return processCSV(r, qctx, meta)
	case "table":
		var r akips.GenericResponse
		if err := r.ParseResponse(body); err != nil {
			return backend.ErrDataResponse(backend.StatusInternal, err.Error())
		}
		return processTable(r, qctx, meta)
	default: // "time_series" and empty string
		var r akips.GenericResponse
		if err := r.ParseResponse(body); err != nil {
			return backend.ErrDataResponse(backend.StatusInternal, err.Error())
		}
		return processTimeSeries(r, qctx, meta)
	}
}

func fieldName(e *akips.GenericResponseEntry) string {
	switch {
	case e.Attribute != "":
		return e.Attribute
	case e.Child != "":
		return e.Child
	default:
		return e.Parent
	}
}

func fieldLabels(e *akips.GenericResponseEntry) data.Labels {
	l := make(data.Labels, 3)
	if e.Parent != "" {
		l["parent"] = e.Parent
	}
	if e.Child != "" {
		l["child"] = e.Child
	}
	if e.Attribute != "" {
		l["attribute"] = e.Attribute
	}
	return l
}

func processTimeSeries(r akips.GenericResponse, q *queryContext, meta *data.FrameMeta) backend.DataResponse {
	var res backend.DataResponse
	if len(r) == 0 {
		return res
	}

	for _, line := range r {
		if len(line.Values) == 0 {
			continue
		}

		datapoints := make([]*float64, len(line.Values))
		for i, v := range line.Values {
			if vv, err := strconv.ParseFloat(v, 64); err == nil {
				datapoints[i] = &vv
			}
		}

		fn := fieldName(line)
		df := data.NewField(fn, fieldLabels(line), datapoints)
		res.Frames = append(res.Frames, &data.Frame{
			Fields: []*data.Field{q.mkTimestampField(len(line.Values)), df},
			Meta:   meta,
			RefID:  q.dq.RefID,
		})
	}
	return res
}

func processTable(r akips.GenericResponse, q *queryContext, meta *data.FrameMeta) backend.DataResponse {
	if len(r) == 0 {
		return backend.DataResponse{}
	}

	vlen := 0
	for _, line := range r {
		if len(line.Values) > vlen {
			vlen = len(line.Values)
		}
	}

	cvt := make([]data.FieldConverter, 0, vlen+3)
	cvt = append(cvt, data.FieldConverter{OutputFieldType: data.FieldTypeString})
	if !q.model.OmitParents {
		cvt = append(cvt,
			data.FieldConverter{OutputFieldType: data.FieldTypeString},
			data.FieldConverter{OutputFieldType: data.FieldTypeString},
		)
	}
	for i := 0; i < vlen; i++ {
		c := converters.AnyToNullableString
		if i < len(r[0].Values) {
			if _, err := strconv.ParseInt(r[0].Values[i], 10, 64); err == nil {
				c = converters.StringToNullableFloat64
			}
		}
		cvt = append(cvt, c)
	}

	builder, err := data.NewFrameInputConverter(cvt, len(r))
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, err.Error())
	}

	names := make([]string, 0, vlen+3)
	if !q.model.OmitParents {
		names = append(names, "Parent", "Child", "Attribute")
	} else {
		names = append(names, "Name")
	}
	for i := 0; i < vlen; i++ {
		names = append(names, fmt.Sprintf("Value #%d", i))
	}
	if err := builder.Frame.SetFieldNames(names...); err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, err.Error())
	}

	for i, line := range r {
		var offset int
		if !q.model.OmitParents {
			_ = builder.Set(0, i, line.Parent)
			_ = builder.Set(1, i, line.Child)
			_ = builder.Set(2, i, line.Attribute)
			offset = 3
		} else {
			_ = builder.Set(0, i, fieldName(line))
			offset = 1
		}
		for fi, v := range line.Values {
			var val interface{}
			if v != "" {
				val = v
			}
			_ = builder.Set(fi+offset, i, val)
		}
	}

	builder.Frame.RefID = q.dq.RefID
	builder.Frame.Meta = meta
	return backend.DataResponse{Frames: data.Frames{builder.Frame}}
}

func processCSV(r akips.CSVResponse, q *queryContext, meta *data.FrameMeta) backend.DataResponse {
	if len(r) == 0 {
		return backend.DataResponse{}
	}

	vlen := 0
	for _, line := range r {
		if len(line) > vlen {
			vlen = len(line)
		}
	}

	cvt := make([]data.FieldConverter, vlen)
	for i := range cvt {
		c := converters.AnyToNullableString
		if i < len(r[0]) {
			if _, err := strconv.ParseInt(r[0][i], 10, 64); err == nil {
				c = converters.StringToNullableFloat64
			}
		}
		cvt[i] = c
	}

	builder, err := data.NewFrameInputConverter(cvt, len(r))
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, err.Error())
	}

	names := make([]string, vlen)
	for i := range names {
		names[i] = fmt.Sprintf("Value #%d", i)
	}
	if err := builder.Frame.SetFieldNames(names...); err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, err.Error())
	}

	for i, line := range r {
		for fi, v := range line {
			var val interface{}
			if v != "" {
				val = v
			}
			_ = builder.Set(fi, i, val)
		}
	}

	builder.Frame.RefID = q.dq.RefID
	builder.Frame.Meta = meta
	return backend.DataResponse{Frames: data.Frames{builder.Frame}}
}

// CheckHealth verifies connectivity and auth against AKIPS.
func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	// 1. Validation
	if d.Settings.URL == "" || d.Settings.Username == "" || d.Settings.Secrets.Password == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Configuration incomplete: URL, Username, or Password missing",
		}, nil
	}

	// 2. Prepare Request
	url := fmt.Sprintf("%s/api-db/", d.Settings.URL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &backend.CheckHealthResult{Status: backend.HealthStatusError, Message: "Failed to create request"}, nil
	}

	// Set Query Params
	hq := httpReq.URL.Query()
	hq.Set("username", d.Settings.Username)
	hq.Set("password", d.Settings.Secrets.Password)
	hq.Set("cmds", "mget * * sysName")
	httpReq.URL.RawQuery = hq.Encode()

	// 3. Execute
	resp, err := d.Client.Do(httpReq)
	if err != nil {
		// Safe error handling (no resp.StatusCode access here)
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Error during client HTTP request to AKIPS endpoint",
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// 4. Validate Response
	if resp.StatusCode != http.StatusOK {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("AKIPS returned error status: %s", resp.Status),
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to read response body from AKIPS",
		}, nil
	}

	if strings.Contains(string(body), "ERROR") {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: string(body),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Success: Connected to AKIPS",
	}, nil
}
