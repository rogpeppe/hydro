package hydroserver

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroreport"
)

// TODO add graph of energy usage and sample count.
var reportTempl = newTemplate(`
<html>
	<head>
		<title>Energy usage report {{.Report.T0.Format "2006-01"}}</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<style type="text/css">
			html, body {
				margin: 0 auto;
				font-family:"Helvetica Neue",Helvetica,Arial,sans-serif;
				font-size:14px;
				color:#333;
				background-color:#fff;
			}
			table {
				border-collapse: collapse;
				border: 1px solid #666666;
				padding-bottom: 3px;
			}
			th, td {
				padding-top: 3px;
				padding-bottom: 3px;
				padding-right: 10px;
				padding-left: 10px;
			}
			.content {margin:10px;}
			tbody tr:nth-child(odd) {
				background-color: #ffffff;
			}

			tbody tr:nth-child(even) {
				background-color: #eeeeee;
			}
		</style>
	</head>
<h2>Energy usage report {{.Report.T0.Format "2006-01"}}</h2>

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
<a href="{{.CSVLink}}" download>Download report CSV</a>
`)

const reportCSVLinkFormat = "hydro-report-2006-01.csv"

func (h *Handler) serveReports(w http.ResponseWriter, req *http.Request) {
	reports := h.store.AvailableReports()
	reportName := strings.TrimPrefix(req.URL.Path, "/reports/")
	if reportName == "" {
		fmt.Fprintf(w, "%d reports available (TODO more info about available reports!)", len(reports))
		return
	}
	wantCSV := false
	tfmt := "2006-01"
	if strings.HasSuffix(reportName, ".csv") {
		wantCSV = true
		tfmt = reportCSVLinkFormat
	}
	t, err := time.ParseInLocation(tfmt, reportName, h.p.TZ)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	for _, report := range reports {
		rt := report.T0
		if rt.Year() == t.Year() && rt.Month() == t.Month() {
			if wantCSV {
				h.serveReportCSV(w, req, report)
			} else {
				h.serveReport(w, req, report)
			}
			return
		}
	}
	http.NotFound(w, req)
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
}

func (h *Handler) serveReport(w http.ResponseWriter, req *http.Request, report *hydroreport.Report) {
	p := reportParams{
		Report:  report,
		CSVLink: fmt.Sprintf("/reports/%s", report.T0.Format(reportCSVLinkFormat)),
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
