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
	Time time.Time
	Reading
}

type result struct {
	index  int
	sample *Sample
}

// GetAll tries to acquire a sample for the meters at all the given
// addresses. If the context is cancelled, it will return immediately
// with the most recent data that it has acquired, which might be from
// an earlier time. The returned slice will hold the result for each
// respective address in addrs. Nil elements will be returned when no
// data has ever been acquired for an address.
func (sampler *Sampler) GetAll(ctx context.Context, addrs ...string) []*Sample {
	results := make(chan result, len(addrs))
	for i, addr := range addrs {
		i, addr := i, addr
		go func() {
			s := sampler.getOne(ctx, addr)
			if s != nil {
				sampler.mu.Lock()
				defer sampler.mu.Unlock()
				sampler.recent[addr] = s
			}
			results <- result{
				index:  i,
				sample: s,
			}
		}()
	}
	samples := make([]*Sample, len(addrs))
	numSamples := 0
	for numSamples < len(samples) {
		select {
		case <-ctx.Done():
			// Fill any samples with previously retrieved data when we have some.
			sampler.mu.Lock()
			defer sampler.mu.Unlock()
			for i, s := range samples {
				if s == nil {
					samples[i] = sampler.recent[addrs[i]]
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
	for ctx.Err() == nil {
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
	}
	log.Printf("failed to get reading from %s: %v", addr, ctx.Err())
	return nil
}
