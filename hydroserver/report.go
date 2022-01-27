package hydroserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rogpeppe/hydro/googlecharts"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroreport"
)

type reportParams struct {
	Report      *hydroreport.Report
	Chargeable  hydroctl.PowerChargeable
	CSVLink     string
	JSONLink    string
	DataColumns []int
}

// TODO add graph of energy usage and sample count.
var reportTempl = newTemplate(`
<html>
	<head>
		<title>Energy usage report {{.Report.Range.T0.Format "2006-01"}}{{if .Report.Partial}} (partial){{end}}</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<link rel="stylesheet" href="/common.css">
		<script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
		<script type="text/javascript">
			google.charts.load('current', {'packages':['corechart']});
			google.charts.setOnLoadCallback(getData);
			function getData() {
				var request = new XMLHttpRequest();
				request.open('GET', '{{.JSONLink}}', true);
				request.onload = function() {
					if (this.status != 200) {
						console.log("got error status", this.status, this.response);
						return
					}
					var data = JSON.parse(this.response);
					drawChart(document, data)
				};
				request.onerror = function() {
					console.log("connection error getting history.json")
				};
				request.send();
			}
			function drawChart(doc, data) {
				var container = document.getElementById('reportGraph');
				var chart = new google.visualization.AreaChart(container);
				var dataTable = new google.visualization.DataTable(data);
				var dataView = new google.visualization.DataView(dataTable);
				var valTransform = function(transform, col) {
					return function(dataTable, row) {
						return transform(dataTable.getValue(row, col))
					}
				}
				// Show the desired columns in the correct order.
				var want = [true, true, true, true, true, true];
				var columns = {{.DataColumns}};
				viewCols = []
				columns.forEach(function(col, i) {
					if(!want[i]) {
						return
					}
					var colType = dataTable.getColumnType(col)
					viewCols.push({
						type: colType,
						calc: function(dataTable, row) {
							var val = dataTable.getValue(row, col);
							if(colType === 'number') {
								val  /= 1000;
							}
							return val;
						},
						label: dataTable.getColumnLabel(col),
					})
				})
				dataView.setColumns(viewCols)

				chart.draw(dataView, {
					title: 'Energy use',
					hAxis: {
						title: 'Time'
					},
					vAxis: {
						minValue: 0,
						title: 'Energy (kWh)'
					},
					hAxis: {
						format: 'MMM d HH:mm',
					},
					explorer: {
						zoomDelta: 1.1,
						maxZoomIn: .04,
						maxZoomOut: 1,
						keepInBounds: true
					},
					isStacked: true
				});
			}
		</script>
	</head>
<h2>Energy usage report {{.Report.Range.T0.Format "2006-01"}}{{if .Report.Partial}} (partial){{end}}</h2>
<a href="{{.CSVLink}}" download>Download report CSV{{if .Report.Partial}} (partial){{end}}</a>
<p/>
{{if .Report.Partial}}Note: this report does not cover the full month. Samples
are only available from {{.Report.Range.T0.Format "2006-01-02"}} to {{.Report.Range.T1.Format "2006-01-02"}}.
{{end}}
<table class="chargeable">
<thead>
	<tr><th>Name</th><th>Chargeable power</th></tr>
</thead>
<tbody>
	<tr><td>Total power exported to grid</td><td>{{.Chargeable.ExportGrid  | kWh}}</td></tr>
	<tr><td>Total export power used by Aliday</td><td>{{.Chargeable.ExportNeighbour | kWh}}</td></tr>
	<tr><td>Total export power used by Drynoch</td><td>{{.Chargeable.ExportHere | kWh}}</td></tr>
	<tr><td>Total import power used by Aliday</td><td>{{.Chargeable.ImportNeighbour | kWh}}</td></tr>
	<tr><td>Total import power used by Drynoch</td><td>{{.Chargeable.ImportHere | kWh}}</td></tr>
</tbody>
</table>
<p/>
<div id="reportGraph" style="height: 600px; width: 800px"></div>
`)

const (
	reportCSVLinkFormat  = "hydro-report-2006-01.csv"
	reportJSONLinkFormat = "2006-01.json"
)

func (h *Handler) serveReports(w http.ResponseWriter, req *http.Request) {
	reports := h.store.AvailableReports()
	reportName := strings.TrimPrefix(req.URL.Path, "/reports/")
	if reportName == "" {
		fmt.Fprintf(w, "%d reports available (TODO more info about available reports!)", len(reports))
		return
	}
	handler := h.serveReport
	tfmt := "2006-01"
	switch {
	case strings.HasSuffix(reportName, ".csv"):
		handler = h.serveReportCSV
		tfmt = reportCSVLinkFormat
	case strings.HasSuffix(reportName, ".json"):
		handler = h.serveReportJSON
		tfmt = reportJSONLinkFormat
	}
	t, err := time.ParseInLocation(tfmt, reportName, h.p.TZ)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	for _, report := range reports {
		rt := report.Range.T0
		if rt.Year() == t.Year() && rt.Month() == t.Month() {
			handler(w, req, report)
			return
		}
	}
	http.NotFound(w, req)
}

var reportGraphLabels = map[string]string{
	"ExportGrid":      "Exported to grid",
	"ExportNeighbour": "Aliday export",
	"ExportHere":      "Drynoch export",
	"ImportNeighbour": "Aliday import",
	"ImportHere":      "Drynoch import",
}

func (h *Handler) serveReportJSON(w http.ResponseWriter, req *http.Request, report *hydroreport.Report) {
	var entries []hydroreport.Entry
	p := report.Params()
	//p.EntryDuration = time.Minute
	r, err := hydroreport.Open(p)
	if err != nil {
		log.Printf("report open failed: %v", err)
		http.Error(w, fmt.Sprintf("cannot open report: %v", err), http.StatusInternalServerError)
		return
	}
	for {
		e, err := r.ReadEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("report template execution failed: %v", err)
			http.Error(w, fmt.Sprintf("cannot get report data points: %v", err), http.StatusInternalServerError)
			return
		}
		entries = append(entries, e)
	}
	table := googlecharts.NewDataTable(entries)
	for id, label := range reportGraphLabels {
		table.Column(id).Label = label
	}
	w.Header().Set("Content-Type", "application/json")
	data, _ := json.Marshal(table)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot marshal data table: %v", err), http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func (h *Handler) serveReportCSV(w http.ResponseWriter, req *http.Request, report *hydroreport.Report) {
	w.Header().Set("Content-Type", "text/csv")
	if err := report.Write(w); err != nil {
		if err != nil {
			log.Printf("error writing report: %v", err)
		}
	}
}

func columnIndex(cols []googlecharts.Column, id string) int {
	for i := range cols {
		if cols[i].ID == id {
			return i
		}
	}
	panic("no column index found for " + id)
}

var columnIndexes = func() []int {
	colIDs := []string{
		"Time",
		"ExportHere",
		"ExportNeighbour",
		"ExportGrid",
		"ImportHere",
		"ImportNeighbour",
	}
	cols := googlecharts.Columns([]hydroreport.Entry(nil))
	indexes := make([]int, len(colIDs))
	for i, id := range colIDs {
		indexes[i] = columnIndex(cols, id)
	}
	return indexes
}()

func (h *Handler) serveReport(w http.ResponseWriter, req *http.Request, report *hydroreport.Report) {
	p := reportParams{
		Report:      report,
		CSVLink:     fmt.Sprintf("/reports/%s", report.Range.T0.Format(reportCSVLinkFormat)),
		JSONLink:    fmt.Sprintf("/reports/%s", report.Range.T0.Format(reportJSONLinkFormat)),
		DataColumns: columnIndexes,
	}

	r, err := hydroreport.Open(report.Params())
	if err != nil {
		log.Printf("report open failed: %v", err)
		http.Error(w, fmt.Sprintf("cannot open report: %v", err), http.StatusInternalServerError)
		return
	}
	for {
		e, err := r.ReadEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("report template execution failed: %v", err)
			http.Error(w, fmt.Sprintf("cannot summarise report: %v", err), http.StatusInternalServerError)
			return
		}
		p.Chargeable = p.Chargeable.Add(e.PowerChargeable)
	}
	var b bytes.Buffer
	if err := reportTempl.Execute(&b, p); err != nil {
		log.Printf("report template execution failed: %v", err)
		http.Error(w, fmt.Sprintf("template execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Write(b.Bytes())
}
