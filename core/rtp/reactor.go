package rtp

import (
	"runtime"
	"sync"
	"time"
)

// Global sharded reactor — escapes Go scheduler limits for 100k concurrent streams.
var MediaReactor *Reactor

func init() {
	MediaReactor = NewReactor()
	MediaReactor.Start()
}

// Tickable represents a media stream or jitter buffer that needs a precise
// 20ms invocation without spawning an independent timer goroutine.
type Tickable interface {
	Tick() bool // return false to signal removal from the reactor
}

// shard owns a partition of the global Tickable slice.
// Each shard runs in its own goroutine, eliminating contention between CPUs.
type shard struct {
	mu    sync.Mutex
	items []Tickable
}

func (s *shard) add(t Tickable) {
	s.mu.Lock()
	s.items = append(s.items, t)
	s.mu.Unlock()
}

// tick processes one 20ms cycle: call every item, prune those returning false.
func (s *shard) tick() {
	s.mu.Lock()
	defer s.mu.Unlock()

	active := s.items[:0]
	for _, item := range s.items {
		if item.Tick() {
			active = append(active, item)
		}
	}
	// Zero out tail to prevent GC-invisible item retention
	for i := len(active); i < len(s.items); i++ {
		s.items[i] = nil
	}
	s.items = active
}

// Reactor is a sharded, multiplexed event loop that drives all RTP send/receive
// timers from a fixed pool of worker goroutines (one per CPU core).
//
// Architecture:
//   Single master timer (20ms) → fan-out goroutine broadcasts to N per-shard channels.
//   Each shard owns a contiguous slice partition → zero cross-shard locking.
//   Items are distributed via round-robin on Add().
//
// Throughput target: ≥100,000 concurrent UDP streams on an 8-core node.
type Reactor struct {
	shards  []*shard
	nShards int
	ticker  *time.Ticker
	quit    chan struct{} // closed by Stop() to terminate the fan-out goroutine
	next    uint64       // round-robin counter (atomic-free; only Add() needs it)
	addMu   sync.Mutex
}

// NewReactor creates a sharded reactor sized to the number of CPU cores.
// Each shard pre-allocates capacity for its share of 100k items.
func NewReactor() *Reactor {
	n := runtime.NumCPU()
	if n < 2 {
		n = 2 // minimum 2 shards even on single-core VMs
	}
	shards := make([]*shard, n)
	perShard := 100000/n + 1
	for i := range shards {
		shards[i] = &shard{
			items: make([]Tickable, 0, perShard),
		}
	}
	return &Reactor{
		shards:  shards,
		nShards: n,
		ticker:  time.NewTicker(20 * time.Millisecond),
		quit:    make(chan struct{}),
	}
}

// Add distributes a Tickable to a shard using round-robin assignment.
// Thread-safe; guaranteed O(1).
func (r *Reactor) Add(t Tickable) {
	r.addMu.Lock()
	idx := r.next % uint64(r.nShards)
	r.next++
	r.addMu.Unlock()

	r.shards[idx].add(t)
}

// Start launches one goroutine per shard plus a fan-out goroutine.
// The fan-out goroutine reads from the single master ticker and signals each
// shard via its own buffered channel — this is a true broadcast, so every
// shard fires exactly once per 20ms tick regardless of read ordering.
func (r *Reactor) Start() {
	// Per-shard tick channels — buffered 1 so the fan-out goroutine is non-blocking.
	fanOut := make([]chan struct{}, r.nShards)
	for i := range fanOut {
		fanOut[i] = make(chan struct{}, 1)
	}

	// Fan-out goroutine: one read from master ticker → one send to every shard.
	go func() {
		for {
			select {
			case <-r.quit:
				for _, ch := range fanOut {
					close(ch)
				}
				return
			case <-r.ticker.C:
				for _, ch := range fanOut {
					select {
					case ch <- struct{}{}:
					default:
						// Shard is still processing the previous tick.
						// Skip rather than block the fan-out goroutine.
					}
				}
			}
		}
	}()

	// One goroutine per shard; each blocks on its own channel.
	for i, s := range r.shards {
		go func(sh *shard, ch <-chan struct{}) {
			for range ch {
				sh.tick()
			}
		}(s, fanOut[i])
	}
}

// Stop halts the master ticker and signals the fan-out goroutine to exit,
// which in turn closes all per-shard channels so shard goroutines drain cleanly.
// Should only be called during process shutdown.
func (r *Reactor) Stop() {
	r.ticker.Stop()
	close(r.quit)
}

// Len returns the total number of active Tickable items across all shards.
// Acquires each shard lock briefly; intended for diagnostics/metrics only.
func (r *Reactor) Len() int {
	total := 0
	for _, s := range r.shards {
		s.mu.Lock()
		total += len(s.items)
		s.mu.Unlock()
	}
	return total
}
