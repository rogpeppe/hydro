package hydroserver

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"gopkg.in/errgo.v1"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/juju/utils/voyeur"
	"github.com/rogpeppe/hydro/hydroctl"
)

type State struct {
	maxCohortId int
	Cohorts     map[string]*Cohort
}

type Cohort struct {
	Id            string // Unique; always increases.
	Index         int    // Display index.
	Title         string
	Relays        []int
	MaxPower      string
	Mode          string
	InUseSlots    []Slot
	NotInUseSlots []Slot
}

type Slot struct {
	Start        string
	SlotDuration string
	Kind         string
	Duration     string
}

type store struct {
	val voyeur.Value

	mu          sync.Mutex
	relayConfig *hydroctl.Config
	state       *State
}

func newStore(initialState *State) (*store, error) {
	cfg, err := parseState(initialState)
	if err != nil {
		return nil, errgo.Notef(err, "bad initial state")
	}
	return &store{
		state:       initialState,
		relayConfig: cfg,
	}, nil
}

func (s *store) Get(path string) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return memState{s.state}.Get(path)
}

func (s *store) Put(path string, val []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(func(st *State) error {
		return memState{st}.Put(path, val)
	})
}

func (s *store) Delete(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(func(st *State) error {
		return memState{st}.Delete(path)
	})
}

// mutate mutates the state atomically, making sure
// that it still makes a valid relay configuration.
// This might turn out to be a bad idea - we may
// need to allow interim illegal states.
func (s *store) mutate(f func(st *State) error) error {
	// TODO do this without json!
	data, err := json.Marshal(&s.state)
	if err != nil {
		return errgo.Notef(err, "cannot marshal state")
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return errgo.Notef(err, "cannot unmarshal state")
	}
	if err := f(&st); err != nil {
		return errgo.Mask(err, errgo.Any)
	}
	cfg, err := parseState(&st)
	if err != nil {
		return errgo.Notef(err, "bad resulting state")
	}
	s.relayConfig = cfg
	s.state = &st
	// Notify any watchers.
	s.val.Set(nil)
	return nil
}

// TODO:
//persistence:
//
//log:
//
//seqno path value
//
//if value is empty, path is deleted (?)
//
//	store.addLog(path, data)

// memState holds a mutable data store. All methods that take
// a path assume that the path is clean
type memState struct {
	state interface{}
}

// Get returns the value corresponding to the given path.
func (s memState) Get(path string) (interface{}, error) {
	pathv, _, err := s.find(path)
	if err != nil {
		return nil, err
	}
	return pathv.Interface(), nil
}

var randKey = func() string {
	data := make([]byte, 8)
	if _, err := rand.Read(data); err != nil {
		panic(fmt.Sprintf("cannot generate random key: %v", err))
	}
	return fmt.Sprintf("%x", data)
}

// TODO
// New creates a new element at the given path,
// which must be a map type with a string key.
// The new element is created with a random key.
//func (s *memState) Create(path string, data []byte) error {
//	pathv, parentv, err := s.find(path)
//}

// Put sets the value of the given path to the given JSON value.
func (s memState) Put(path string, data []byte) error {
	pathv, _, err := s.find(path)
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

// Delete deletes the given path element, which
// must refer to an element of a map.
func (s memState) Delete(path string) error {
	_, parentv, err := s.find(path)
	if err != nil {
		return err
	}
	if parentv.Kind() != reflect.Map || parentv.Type().Key() != reflect.TypeOf("") {
		return fmt.Errorf("can only delete from a string-keyed map, not %s", parentv.Type())
	}
	parentv.SetMapIndex(reflect.ValueOf(lastElem(path)), reflect.Value{})
	return nil
}

func lastElem(path string) string {
	if i := strings.LastIndex(path, "/"); i != -1 {
		return path[i+1:]
	}
	return path
}

// find returns the value of the element at the given path,
// and the value of its parent element.
func (s *memState) find(path string) (reflect.Value, reflect.Value, error) {
	elems := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if elems[len(elems)-1] == "" {
		elems = elems[0 : len(elems)-1]
	}
	var parentv reflect.Value
	statev := reflect.ValueOf(s.state)
	for i, e := range elems {
		var err error
		oldStatev := statev
		statev, parentv, err = walk(statev, e)
		if err != nil {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("cannot get %s in %T: %v", strings.Join(elems[0:i+1], "/"), oldStatev.Type(), err)
		}
		if !statev.IsValid() {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("invalid value %s in %T: %v", strings.Join(elems[0:i+1], "/"), oldStatev.Type(), err)
		}
	}
	return statev, parentv, nil
}

// walk walks into the given element in xv, and returns the
// value found, and its parent value, which may not be the
// same as xv - for example if xv holds a pointer to a struct
// the returned parent will be the struct itself, not
// the pointer.
func walk(xv reflect.Value, elem string) (foundv, parentv reflect.Value, _ error) {
	if !xv.IsValid() {
		return reflect.Value{}, reflect.Value{}, fmt.Errorf("invalid value at %v", elem)
	}
	t := xv.Type()
	switch t.Kind() {
	case reflect.Ptr:
		if xv.IsNil() {
			return reflect.Value{}, reflect.Value{}, fmt.Errorf("found nil pointer")
		}
		return walk(xv.Elem(), elem)
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
