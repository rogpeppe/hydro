package meterstore

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	errgo "gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/meterstore/internal/meterstorepb"
)

type Store struct {
	db     *bolt.DB
	mu     sync.Mutex
	meters []*meterstorepb.MeterInfo
}

type meterInfo struct {
	id       int
	addr     string
	location int
}

type Reading int

var (
	timeRecordBucket = []byte("timerecord")
	meterBucket      = []byte("meter")
)

// meterKey holds the key of the meter info data.
// There's only one meter record in the database that
// holds information for all meters.
var meterKey = []byte{0}

const (
	SystemPower Reading = 1 << iota
	SystemEnergy
	// etc

	MaxReading
)

type TimeRecord struct {
	Time          time.Time
	InLog         bool
	MeterAddr     string
	MeterLocation int
	Readings      Reading
	SystemPower   float64
	SystemEnergy  float64
}

func (r *TimeRecord) setReadings(r1 TimeRecord) {
	if r1.Readings&SystemPower != 0 {
		r.SystemPower = r1.SystemPower
	}
	if r1.Readings&SystemEnergy != 0 {
		r.SystemEnergy = r1.SystemEnergy
	}
}

func Open(file string) (*Store, error) {
	db, err := bolt.Open(file, 0666, nil)
	if err != nil {
		return nil, errgo.Mask(err)
	}
	store := &Store{
		db: db,
	}
	if err := db.Update(store.init); err != nil {
		return nil, errgo.Mask(err)
	}
	return store, nil
}

func (s *Store) init(tx *bolt.Tx) error {
	b, err := tx.CreateBucketIfNotExists(timeRecordBucket)
	if err != nil {
		return errgo.Mask(err)
	}
	b.FillPercent = 75 // Mostly append-only.

	b, err = tx.CreateBucket(meterBucket)
	if err != nil {
		if err != bolt.ErrBucketExists {
			return errgo.Mask(err)
		}
		meters, err := s.getMeterInfo(tx)
		if err != nil {
			return errgo.Mask(err)
		}
		s.meters = meters
	}
	return nil
}

// Close closes the store. All iterators must have been
// closed before calling Close.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return errgo.Mask(err)
}

// Add adds the given time records to the store.
// It must not be called in the same goroutine
// as a current iterator.
func (s *Store) Add(records ...TimeRecord) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(timeRecordBucket)
		if b == nil {
			return errgo.Newf("no time record bucket")
		}
		for _, r := range records {
			key, val, err := s.timeRecordKeyVal(tx, r)
			if err != nil {
				return errgo.Mask(err)
			}
			oldVal := b.Get(key)
			if oldVal == nil {
				// Record doesn't exist, so add a new one.
				b.Put(key, val)
				continue
			}
			var old TimeRecord
			if err := s.unmarshalTimeRecordVal(oldVal, &old); err != nil {
				return errgo.Notef(err, "cannot unmarshal old record")
			}
			old.setReadings(r)
			key, val, err = s.timeRecordKeyVal(tx, old)
			if err != nil {
				return errgo.Mask(err)
			}
			b.Put(key, val)
		}
		return nil
	})
	return errgo.Mask(err)
}

func (s *Store) unmarshalTimeRecordVal(v []byte, r *TimeRecord) error {
	var pbr meterstorepb.TimeRecord
	if err := pbr.UnmarshalBinary(v); err != nil {
		return errgo.Notef(err, "cannot unmarshal record")
	}
	*r = TimeRecord{
		Time:     stampToTime(pbr.Timestamp),
		InLog:    pbr.InLog,
		Readings: Reading(pbr.Readings),
		// TODO correct scaling
		SystemPower:  float64(pbr.SystemPower),
		SystemEnergy: float64(pbr.SystemEnergy),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if int(pbr.MeterId) >= len(s.meters) {
		r.MeterAddr = fmt.Sprintf("unknown%d", pbr.MeterId)
	} else {
		m := s.meters[pbr.MeterId]
		r.MeterAddr = m.Addr
		r.MeterLocation = int(m.Location)
	}
	return nil
}

// timeRecordKey returns a key for the given record. If tx is non-nil,
// it must represent a read-write transaction and the meter
// id will be added if not already present.
func (s *Store) timeRecordKeyVal(tx *bolt.Tx, r TimeRecord) (k, v []byte, err error) {
	// 8 bytes: time in milliseconds uint (big-endian)
	// 1 byte: in log
	// 1 byte: meter id

	k = make([]byte, 8+2)
	binary.BigEndian.PutUint64(k, timeToStamp(r.Time))
	if r.InLog {
		k[8] = 1
	}
	id, err := s.meterId(tx, r.MeterAddr, r.MeterLocation)
	if err != nil {
		return nil, nil, errgo.Mask(err)
	}
	k[9] = id

	pbr := meterstorepb.TimeRecord{
		Timestamp: timeToStamp(r.Time),
		InLog:     r.InLog,
		MeterId:   uint32(id),
		Readings:  uint32(r.Readings),
		// TODO sort out correct scaling
		SystemPower:  int32(r.SystemPower),
		SystemEnergy: int32(r.SystemEnergy),
	}
	v, err = pbr.MarshalBinary()
	if err != nil {
		return nil, nil, errgo.Mask(err)
	}
	return k, v, nil
}

// timeKey returns a key that finds a record by time
// without choosing any keys. Note that this must be
// kept in sync with Store.key.
func timeKey(t time.Time) []byte {
	k := make([]byte, 8+2)
	binary.BigEndian.PutUint64(k, timeToStamp(t))
	return k
}

func (s *Store) meterId(tx *bolt.Tx, addr string, loc int) (uint8, error) {
	if addr == "" {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.meters {
		m := s.meters[i]
		if m.Addr == addr && int(m.Location) == loc {
			return uint8(i), nil
		}
	}
	if tx == nil {
		return 0, errgo.Newf("no id found for meter %q at location %v", addr, loc)
	}
	if len(s.meters) >= 254 {
		panic("out of meter ids!")
	}
	s.meters = append(s.meters, &meterstorepb.MeterInfo{
		Addr:     addr,
		Location: int32(loc),
	})
	info := meterstorepb.MeterRecord{
		Meters: s.meters,
	}
	data, err := info.MarshalBinary()
	if err != nil {
		return 0, errgo.Notef(err, "cannot marshal meter info")
	}
	if err := tx.Bucket(meterBucket).Put(meterKey, data); err != nil {
		return 0, errgo.Notef(err, "cannot update meter info")
	}
	return uint8(len(s.meters) - 1), nil
}

// getMeterInfo gets the meter info from the database.
// It expects to be called with s.mu held.
func (s *Store) getMeterInfo(tx *bolt.Tx) ([]*meterstorepb.MeterInfo, error) {
	b := tx.Bucket(meterBucket)
	if b == nil {
		return nil, errgo.Newf("no meter bucket found")
	}
	data := b.Get(meterKey)
	if data == nil {
		return nil, nil
	}
	var info meterstorepb.MeterRecord
	if err := info.UnmarshalBinary(data); err != nil {
		return nil, errgo.Notef(err, "cannot unmarshal meter data")
	}
	return info.Meters, nil
}

// Iter returns an iterator that starts at the given time
// and returns records in time order from then.
func (s *Store) Iter(start time.Time) *Iter {
	return s.iter(start, true)
}

func (s *Store) ReverseIter(start time.Time) *Iter {
	return s.iter(start, false)
}

func (s *Store) iter(start time.Time, forward bool) *Iter {
	tx, err := s.db.Begin(false)
	if err != nil {
		return &Iter{
			error: errgo.Notef(err, "cannot begin transaction"),
		}
	}
	b := tx.Bucket(timeRecordBucket)
	if b == nil {
		return &Iter{
			error: errgo.Newf("time record bucket not found"),
		}
	}
	return &Iter{
		store:    s,
		tx:       tx,
		cursor:   b.Cursor(),
		forward:  forward,
		needSeek: true,
		seekTo:   start,
	}
}

type Iter struct {
	store    *Store
	tx       *bolt.Tx
	cursor   *bolt.Cursor
	forward  bool
	seekTo   time.Time
	needSeek bool
	current  TimeRecord
	error    error
}

// Iter advances to the next item in the iteration.

func (i *Iter) Next() bool {
	if i.cursor == nil {
		return false
	}
	// nextIfAfter holds whether we need to skip over the
	// first record if we find one that's not after the time we've
	// seeked to. This happens only when we're seeking backwards
	// because the bolt Seek semantics find the record *after*
	// the one we're looking for, so we want to iterate backwards
	// from t=30 and there are only records t=25 and t=35, we
	// don't want to include t=35, so we skip one back.
	nextIfAfter := false
	var foundKey, foundValue []byte
	switch {
	case i.needSeek && !i.seekTo.IsZero():
		key := timeKey(i.seekTo)
		if !i.forward {
			// Try to find one beyond the last record with the
			// requested time stamp.
			key[9] = 255
		}
		foundKey, foundValue = i.cursor.Seek(key)
		if !i.forward {
			// If we're going backwards, start with the record
			// after where we're looking for, then return the
			// one before that.
			if foundKey == nil {
				foundKey, foundValue = i.cursor.Last()
			}
			nextIfAfter = true
		}
	case i.needSeek && i.forward:
		foundKey, foundValue = i.cursor.First()
	case i.needSeek && !i.forward:
		foundKey, foundValue = i.cursor.Last()
	case i.forward:
		foundKey, foundValue = i.cursor.Next()
	default:
		foundKey, foundValue = i.cursor.Prev()
	}
	i.needSeek = false
	if foundKey == nil {
		i.cursor = nil
		if err := i.tx.Rollback(); err != nil {
			i.error = errgo.Notef(err, "cannot close transaction")
		}
		return false
	}
	if err := i.store.unmarshalTimeRecordVal(foundValue, &i.current); err != nil {
		i.cursor = nil
		i.error = errgo.Mask(err)
		// Ignore any close error - we've already got one.
		i.Close()
		return false
	}
	if nextIfAfter && i.seekTo.Before(i.current.Time) {
		return i.Next()
	}
	return true
}

func (i *Iter) Close() error {
	i.cursor = nil
	if i.tx == nil {
		return errgo.Mask(i.error)
	}
	if err := i.tx.Rollback(); err != nil {
		if i.error == nil {
			i.error = errgo.Mask(err)
		}
	}
	i.tx = nil
	return errgo.Mask(i.error)
}

func (i *Iter) Value() TimeRecord {
	return i.current
}

func (i *Iter) Err() error {
	return i.error
}

func timeToStamp(t time.Time) uint64 {
	return uint64(t.Round(time.Millisecond).UnixNano() / int64(time.Millisecond))
}

func stampToTime(t uint64) time.Time {
	return time.Unix(0, int64(t)*int64(time.Millisecond))
}
