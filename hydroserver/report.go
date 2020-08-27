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

// TODO add graph of energy usage and sample count.
var reportTempl = newTemplate(`
<html>
	<head>
		<title>Energy usage report {{.Report.T0.Format "2006-01"}}</title>
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
				chart.draw(dataTable, {
					title: 'Energy use',
					hAxis: {
						title: 'Time'
					},
					vAxis: {
						minValue: 0,
						title: 'Energy (kWh)'
					}
				});
			}
		</script>
	</head>
<h2>Energy usage report {{.Report.T0.Format "2006-01"}}</h2>
<a href="{{.CSVLink}}" download>Download report CSV</a>
<p/>
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
		rt := report.T0
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

type reportParams struct {
	Report     *hydroreport.Report
	Chargeable hydroctl.PowerChargeable
	CSVLink    string
	JSONLink   string
}

func (h *Handler) serveReport(w http.ResponseWriter, req *http.Request, report *hydroreport.Report) {
	p := reportParams{
		Report:   report,
		CSVLink:  fmt.Sprintf("/reports/%s", report.T0.Format(reportCSVLinkFormat)),
		JSONLink: fmt.Sprintf("/reports/%s", report.T0.Format(reportJSONLinkFormat)),
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
