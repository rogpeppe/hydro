package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func getVal(x interface{}, path string) (interface{}, error) {
	pathVal, _, err := find(x, path)
	if err != nil {
		return nil, err
	}
	return pathVal.Interface(), nil
}

func putVal(x interface{}, path string, data []byte) error {
	pathv, _, err := find(x, path)
	if err != nil {
		return err
	}
	if !pathv.CanSet() {
		return fmt.Errorf("cannot set value")
	}
	// Make sure that we entirely overwrite the value by unmarshaling
	// into a freshly made zero value rather than the existing value.
	destv := reflect.New(pathv.Type())
	if err := json.Unmarshal(data, destv.Interface()); err != nil {
		return fmt.Errorf("cannot unmarshal %q into %s: %v", data, pathv.Type(), err)
	}
	pathv.Set(destv.Elem())
	return nil
}

func deleteVal(x interface{}, path string) error {
	_, parentv, err := find(x, path)
	if err != nil {
		return err
	}
	if parentv.Kind() != reflect.Map || parentv.Type().Key() != reflect.TypeOf("") {
		return fmt.Errorf("can only delete from a string-keyed map, not %s", parentv.Type())
	}
	lastSlash := strings.LastIndex(path, "/")
	var lastElem string
	if lastSlash == -1 {
		lastElem = path
	} else {
		lastElem = path[lastSlash+1:]
	}
	parentv.SetMapIndex(reflect.ValueOf(lastElem), reflect.Value{})
	return nil
}

func find(x interface{}, path string) (reflect.Value, reflect.Value, error) {
	elems := strings.Split(path, "/")
	if elems[len(elems)-1] == "" {
		elems = elems[0 : len(elems)-1]
	}
	var parentv reflect.Value
	xv := reflect.ValueOf(x)
	for i, e := range elems {
		var err error
		xv, parentv, err = descend(xv, e)
		if err != nil {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("cannot get %s: %v", strings.Join(elems[0:i+1], "/"), err)
		}
	}
	return xv, parentv, nil
}

func descend(xv reflect.Value, elem string) (foundv, parentv reflect.Value, _ error) {
	t := xv.Type()
	switch t.Kind() {
	case reflect.Ptr:
		if xv.IsNil() {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("found nil pointer")
		}
		return descend(xv.Elem(), elem)
	case reflect.Struct:
		f, ok := t.FieldByName(elem)
		if !ok {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("field %s not found", elem)
		}
		return xv.FieldByIndex(f.Index), xv, nil
	case reflect.Slice, reflect.Array:
		i, err := strconv.Atoi(elem)
		if err != nil {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("bad index value %q into slice %s", elem, t)
		}
		if i < 0 || i >= xv.Len() {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("index %d out of range in %s", i, t)
		}
		return xv.Index(i), xv, nil
	case reflect.Map:
		if t.Key() != reflect.TypeOf("") {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("non-string-keyed map %s", t)
		}
		elem := xv.MapIndex(reflect.ValueOf(elem))
		if !elem.IsValid() {
			elem = reflect.Zero(t.Elem())
		}
		return elem, xv, nil
	case reflect.Interface:
		if xv.IsNil() {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("found nil interface")
		}
		return xv.Elem(), xv, nil
	}
	return reflect.Value{}, reflect.Value{}, fmt.Errorf("cannot find %q in type %s", elem, t)
}
