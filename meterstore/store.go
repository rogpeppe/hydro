package meterstore

import (
	"encoding/binary"
	"time"

	"github.com/boltdb/bolt"
	"github.com/rogpeppe/hydro/meterstore/internal/meterstorepb"
	errgo "gopkg.in/errgo.v1"
)

type Store struct {
	db *bolt.DB
}

type Reading int

const (
	SystemPower Reading = 1 << iota
	SystemEnergy
	// etc

	MaxReading
)

type table byte

const (
	readingsTable table = iota * 2
	configTable
	historyTable
)

func timeToStamp(t time.Time) uint64 {
	return uint64(t.Round(time.Millisecond).UnixNano() / int64(time.Millisecond))
}

func stampToTime(t uint64) time.Time {
	return time.Unix(0, int64(t)*int64(time.Millisecond))
}

func (r TimeRecord) key() []byte {
	// 8 bytes: time in milliseconds uint (big-endian)
	// 1 byte: in log
	// 1 byte: meter id

	k := make([]byte, 8+2)
	binary.BigEndian.PutUint64(k, timeToStamp(r.Time))
	if r.InLog {
		k[8] = 1
	}
	k[9] = byte(r.Meter)
	return k
}

func (r TimeRecord) data() []byte {
	pbr := meterstorepb.TimeRecord{
		Timestamp: timeToStamp(r.Time),
		InLog:     r.InLog,
		Meter:     uint32(r.Meter),
		Readings:  uint32(r.Readings),
		// TODO sort out correct scaling
		SystemPower:  int32(r.SystemPower/100 + 0.5),
		SystemEnergy: int32(r.SystemEnergy + 0.5),
	}
	data, err := pbr.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return data
}

func (r *TimeRecord) unmarshalBinary(data []byte) error {
	var pbr meterstorepb.TimeRecord
	if err := pbr.UnmarshalBinary(data); err != nil {
		return errgo.Notef(err, "cannot unmarshal record")
	}
	*r = TimeRecord{
		Time:     time.Unix(0, int64(pbr.Timestamp*1000)),
		InLog:    pbr.InLog,
		Meter:    int(pbr.Meter),
		Readings: Reading(pbr.Readings),
		// TODO correct scaling
		SystemPower:  float64(pbr.SystemPower) * 100,
		SystemEnergy: float64(pbr.SystemEnergy) * 100,
	}
	return nil
}

type TimeRecord struct {
	Time         time.Time
	InLog        bool
	Meter        int
	Readings     Reading
	SystemPower  float64
	SystemEnergy float64
}

var (
	timeRecordBucket = []byte("timerecord")
)

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
	return nil
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
			key := r.key()
			oldVal := b.Get(key)
			if oldVal == nil {
				// Record doesn't exist, so add a new one.
				b.Put(key, r.data())
				continue
			}
			var old TimeRecord
			if err := old.unmarshalBinary(oldVal); err != nil {
				return errgo.Notef(err, "cannot unmarshal old record")
			}
			old.setReadings(r)
			b.Put(key, old.data())
		}
		return nil
	})
	return errgo.Mask(err)
}

func (r *TimeRecord) setReadings(r1 TimeRecord) {
	if r1.Readings&SystemPower != 0 {
		r.SystemPower = r1.SystemPower
	}
	if r1.Readings&SystemEnergy != 0 {
		r.SystemEnergy = r1.SystemEnergy
	}
}

// TimeIter returns an iterator that starts at the given time
// and returns records in time order from then.
func (s *Store) TimeIter(start time.Time) *Iter {
	return s.iter(start, true)
}

func (s *Store) ReverseTimeIter(start time.Time) *Iter {
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
		tx:       tx,
		cursor:   b.Cursor(),
		forward:  true,
		needSeek: true,
		seekTo:   start,
	}
}

type Iter struct {
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
	var foundKey, foundValue []byte
	switch {
	case i.needSeek && !i.seekTo.IsZero():
		key := TimeRecord{Time: i.seekTo}.key()
		if !i.forward {
			// Try to find one beyond the last record with the
			// requested time stamp.
			key[9] = 255
		}
		foundKey, foundValue = i.cursor.Seek(key)
		if foundKey == nil {
			foundKey, foundValue = i.cursor.Last()
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
	if err := i.current.unmarshalBinary(foundValue); err != nil {
		i.cursor = nil
		i.error = errgo.Mask(err)
		// Ignore the error - we've already got one.
		i.Close()
		return false
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
