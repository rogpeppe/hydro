package ndmeter

import (
	"context"
	"log"
	"sync"
	"time"

	"go4.org/syncutil/singleflight"
)

func NewSampler() *Sampler {
	return &Sampler{
		recent: make(map[string]*Sample),
	}
}

// Sampler allows the sampling of a set of meters over time.
type Sampler struct {
	group  singleflight.Group
	mu     sync.Mutex
	recent map[string]*Sample
}

// Sample holds a meter reading that was received at
// a particular time.
type Sample struct {
	// Time holds the time that the reading was received.
	Time time.Time
	Reading
}

type result struct {
	index  int
	sample *Sample
}

// SamplePlace holds where to get a sample from.
type SamplePlace struct {
	// Addr holds the address of the meter to contact to get the sample.
	Addr string

	// AllowedLag holds the allowed lag of a sample that GetAll may
	// return. If there's a sample that's already available and not
	// more than AllowedLag old when GetAll is called, it will be
	// returned instead of making a new call.
	AllowedLag time.Duration
}

// GetAll tries to acquire a sample for the meters at all the given
// addresses. If the context is cancelled, it will return immediately
// with the most recent data that it has acquired, which might be from
// an earlier time. The returned slice will hold the result for each
// respective address in addrs. Nil elements will be returned when no
// data has ever been acquired for an address.
func (sampler *Sampler) GetAll(ctx context.Context, places ...SamplePlace) []*Sample {
	results := make(chan result, len(places))
	samples := make([]*Sample, len(places))
	numSamples := 0
	for i, place := range places {
		i, place := i, place

		sampler.mu.Lock()
		oldSample, ok := sampler.recent[place.Addr]
		sampler.mu.Unlock()
		var lag time.Duration
		if ok {
			lag = time.Since(oldSample.Time)
			if lag < place.AllowedLag {
				// We already have a sample that's within the allowed lag, so use that.
				samples[i] = oldSample
				numSamples++
				continue
			}
		}
		go func() {
			s := sampler.getOne(ctx, place.Addr)
			if s != nil {
				sampler.mu.Lock()
				defer sampler.mu.Unlock()
				sampler.recent[place.Addr] = s
			}
			results <- result{
				index:  i,
				sample: s,
			}
		}()
	}
	for numSamples < len(samples) {
		select {
		case <-ctx.Done():
			// Fill any samples with previously retrieved data when we have some.
			sampler.mu.Lock()
			defer sampler.mu.Unlock()
			for i, s := range samples {
				if s == nil {
					samples[i] = sampler.recent[places[i].Addr]
				}
			}
			return samples
		case s := <-results:
			samples[s.index] = s.sample
			numSamples++
		}
	}
	return samples
}

func (sampler *Sampler) getOne(ctx context.Context, addr string) *Sample {
	retry := 100 * time.Millisecond
	for ctx.Err() == nil {
		t0 := time.Now()
		sample0, err := sampler.group.Do(addr, func() (interface{}, error) {
			reading, err := Get(addr)
			return &Sample{
				Time:    time.Now(),
				Reading: reading,
			}, err
		})
		sample := sample0.(*Sample)
		if err == nil {
			return sample
		}

		log.Printf("failed to get reading from %s: %v", addr, err)
		if !isTemporary(err) {
			// Don't retry on non-temporary errors
			return nil
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Until(t0.Add(retry))):
		}
	}
	log.Printf("context done reading from %s: %v", addr, ctx.Err())
	return nil
}

type temporary interface {
	Temporary() bool
}

func isTemporary(err error) bool {
	t, ok := err.(temporary)
	return ok && t.Temporary()
}
