package googlecharts

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	errgo "gopkg.in/errgo.v1"
)

// DataTable holds the contents of a data table. When marshaled as JSON,
// it is suitable for passing to dataview.fromJSON.
type DataTable struct {
	Cols []Column `json:"cols"`
	Rows []Row    `json:"rows"`
}

type Column struct {
	Type    DataType `json:"type"`
	Id      string   `json:"id"`
	Label   string   `json:"label,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
}

type Row struct {
	Cells      []Cell                 `json:"c"`
	Properties map[string]interface{} `json:"p,omitempty"`
}

type Cell struct {
	Value      interface{}            `json:"v,omitempty"`
	Format     string                 `json:"f,omitempty"`
	Properties map[string]interface{} `json:"p,omitempty"`
}

type DataType string

const (
	TBool      = "boolean"
	TNumber    = "number"
	TString    = "string"
	TDate      = "date"
	TDatetime  = "datetime"
	TTimeofday = "timeofday"
)

type tableType struct {
	build func(xv reflect.Value) *DataTable
}

// NewDataTable returns a new data table by taking values from x, which
// must be a slice of a struct type or pointer to struct type.
// TODO allow map[string]structT too?
//
// The fields of the struct type determine the columns in the table;
// their types determine the type of the coloumn. Any numeric type is
// given the type "number", boolean types are "boolean", string types
// are "string", A value of type time.Time is encoded as a "datetime"
// column by default, but that can be changed using the struct tag
// options (see below). A value of type Duration encodes as "timeofday".
// TODO is the above a good idea?
//
// The id of the column is taken from the field name by default. The
// other column values can be customized by the format string stored
// under the "googlecharts" json key in the struct field's tags. The
// format string gives the label of the field, possibly followed by a
// comma-separated list of options. The label may be empty to leave the
// label unspecified.
//
// The "type" option specifies the field type. This is usually inferred
// from the type.
//
// The "id" option specifies the id of the column.
func NewDataTable(x interface{}) *DataTable {
	xv := reflect.ValueOf(x)
	info, err := getTypeInfo(xv.Type())
	if err != nil {
		panic(err)
	}
	nrows := xv.Len()
	dt := DataTable{
		Cols: append([]Column(nil), info.cols...),
		Rows: make([]Row, nrows),
	}
	cells := make([]Cell, nrows*len(info.cols))
	for row := range dt.Rows {
		elemv := xv.Index(row)
		rcells := cells[0:len(info.cols):len(info.cols)]
		dt.Rows[row].Cells = rcells
		cells = cells[len(info.cols):]
		if info.indir {
			if elemv.IsNil() {
				continue
			}
			elemv = elemv.Elem()
		}
		for col := range rcells {
			info := &info.fields[col]
			info.set(&rcells[col], elemv.FieldByIndex(info.index))
		}
	}
	return &dt
}

type typeInfo struct {
	indir  bool
	cols   []Column
	fields []fieldInfo
}

var (
	typeMutex sync.RWMutex
	typeMap   = make(map[reflect.Type]*typeInfo)
)

func getTypeInfo(t reflect.Type) (*typeInfo, error) {
	typeMutex.RLock()
	pt := typeMap[t]
	typeMutex.RUnlock()
	if pt != nil {
		return pt, nil
	}
	typeMutex.Lock()
	defer typeMutex.Unlock()
	if pt = typeMap[t]; pt != nil {
		// The type has been parsed after we dropped
		// the read lock, so use it.
		return pt, nil
	}
	pt, err := parseTypeInfo(t)
	if err != nil {
		return nil, errgo.Mask(err)
	}
	typeMap[t] = pt
	return pt, nil
}

func parseTypeInfo(xt reflect.Type) (*typeInfo, error) {
	if xt.Kind() != reflect.Slice {
		return nil, errgo.Newf("argument to NewDataTable needs slice, got %v", xt)
	}
	t := xt.Elem()
	var info typeInfo
	if t.Kind() == reflect.Ptr {
		info.indir = true
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, errgo.Newf("argument to NewDataTable needs []struct or []*struct, got %v", xt)
	}
	info.fields = make([]fieldInfo, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		fi, err := getFieldInfo(f)
		if err != nil {
			return nil, errgo.Mask(err)
		}
		info.fields = append(info.fields, fi)
		info.cols = append(info.cols, Column{
			Id:    fi.id,
			Label: fi.label,
			Type:  fi.dtype,
		})
	}
	return &info, nil
}

var kindToDataType = map[reflect.Kind]DataType{
	reflect.Bool:    TBool,
	reflect.Int:     TNumber,
	reflect.Int8:    TNumber,
	reflect.Int16:   TNumber,
	reflect.Int32:   TNumber,
	reflect.Int64:   TNumber,
	reflect.Uint:    TNumber,
	reflect.Uint8:   TNumber,
	reflect.Uint16:  TNumber,
	reflect.Uint32:  TNumber,
	reflect.Uint64:  TNumber,
	reflect.Uintptr: TNumber,
	reflect.Float32: TNumber,
	reflect.Float64: TNumber,
	reflect.String:  TString,
}

type fieldInfo struct {
	id    string
	label string
	index []int
	dtype DataType
	set   func(cell *Cell, xv reflect.Value)
}

var timeType = reflect.TypeOf(time.Time{})

func getFieldInfo(f reflect.StructField) (fieldInfo, error) {
	dt, ok := kindToDataType[f.Type.Kind()]
	if !ok {
		if f.Type != timeType {
			return fieldInfo{}, errgo.Newf("type %s not allowed for field %v", f.Type, f.Name)
		}
		dt = TDatetime
	}
	info := fieldInfo{
		id:    f.Name,
		dtype: dt,
		index: f.Index,
		set: func(cell *Cell, xv reflect.Value) {
			cell.Value = xv.Interface()
		},
	}
	if dt == TDatetime {
		info.set = func(cell *Cell, xv reflect.Value) {
			t := xv.Interface().(time.Time)
			cell.Value = fmt.Sprintf("Date(%d)", t.UnixNano()/1e6)
		}
	}
	tag := f.Tag.Get("googlecharts")
	if tag == "" {
		return info, nil
	}
	parts := strings.SplitN(tag, ",", 2)
	info.label = parts[0]
	if len(parts) == 1 {
		return info, nil
	}
	// TODO options
	return info, nil
}
