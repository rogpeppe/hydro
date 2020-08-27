package hydroserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rogpeppe/hydro/googlecharts"
	"github.com/rogpeppe/hydro/hydroctl"
)

type historyRecord struct {
	Name  string
	Start time.Time
	End   time.Time
}

func (h *Handler) serveHistory(w http.ResponseWriter, req *http.Request) {
	ws := h.store.WorkerState()
	if ws == nil {
		http.Error(w, "no current relay information available", http.StatusInternalServerError)
		return
	}
	cfg := h.store.CtlConfig()
	now := time.Now()
	offTimes := make([]time.Time, hydroctl.MaxRelayCount)
	for i := range offTimes {
		if ws.State.IsSet(i) {
			offTimes[i] = now
		}
	}
	limit := now.Add(-7 * 24 * time.Hour)
	var records []historyRecord
	iter := h.history.ReverseIter()
	for iter.Next() {
		e := iter.Item()
		if e.Time.Before(limit) {
			break
		}
		if e.On {
			if offt := offTimes[e.Relay]; !offt.IsZero() {
				records = append(records, historyRecord{
					// TODO use relay number only when needed for disambiguation.
					Name:  fmt.Sprintf("%d: %s", e.Relay, cfg.Relays[e.Relay].Cohort),
					Start: e.Time,
					End:   offt,
				})
				offTimes[e.Relay] = time.Time{}
			}
		} else {
			offTimes[e.Relay] = e.Time
		}
	}
	// Give starting times to all the periods that start before the limit.
	for i, offt := range offTimes {
		if !offt.IsZero() {
			records = append(records, historyRecord{
				// TODO use relay number only when needed for disambiguation.
				Name:  fmt.Sprintf("%d: %s", i, cfg.Relays[i].Cohort),
				Start: limit,
				End:   offt,
			})
		}
	}
	data, err := json.Marshal(googlecharts.NewDataTable(records))
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot marshal data table: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
