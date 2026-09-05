package sdl_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// TestMultiplexerConcurrentNext drives one multiplexer from many goroutines.
//
// Before the mutex, Multiplexer mutated currentIndex and remainingWeight with
// no lock at all while Channel carried one, so the scheduler state raced even
// though the buffers did not. Run with -race.
func TestMultiplexerConcurrentNext(t *testing.T) {
	const channels = 4
	const perChannel = 250

	mux := sdl.NewMultiplexer[int]()
	for id := range channels {
		ch := sdl.NewChannel[int](uint8(id), perChannel)
		for n := range perChannel {
			if err := ch.Add(id*perChannel + n); err != nil {
				t.Fatal(err)
			}
		}
		mux.AddChannel(ch, id+1)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := make(map[int]int)

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				frame, err := mux.Next()
				if err != nil {
					return
				}
				mu.Lock()
				seen[frame]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Every frame must come out exactly once: no duplicates from a torn
	// cursor, no losses from a check-then-act gap.
	if len(seen) != channels*perChannel {
		t.Errorf("got %d distinct frames, want %d", len(seen), channels*perChannel)
	}
	for frame, count := range seen {
		if count != 1 {
			t.Errorf("frame %d delivered %d times, want once", frame, count)
		}
	}
}

// TestMultiplexerConcurrentAddAndNext mixes registration with scheduling,
// which is what races the sortedIDs slice and the priority map.
func TestMultiplexerConcurrentAddAndNext(t *testing.T) {
	mux := sdl.NewMultiplexer[int]()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for id := range 16 {
			ch := sdl.NewChannel[int](uint8(id), 8)
			for n := range 8 {
				_ = ch.Add(id*8 + n)
			}
			mux.AddChannel(ch, 1)
		}
	}()

	go func() {
		defer wg.Done()
		for range 200 {
			if _, err := mux.Next(); err != nil && !errors.Is(err, sdl.ErrNoFramesAvailable) &&
				!errors.Is(err, sdl.ErrNoChannels) {
				t.Errorf("unexpected error: %v", err)
				return
			}
			_ = mux.HasPending()
			_ = mux.Len()
		}
	}()

	wg.Wait()
}

// mcSource is a master channel standing in for the real ones.
type mcSource struct {
	scid uint16
	ch   *sdl.Channel[int]
}

func (m *mcSource) SCID() uint16               { return m.scid }
func (m *mcSource) GetNextFrame() (int, error) { return m.ch.Next() }
func (m *mcSource) HasPendingFrames() bool     { return m.ch.HasFrames() }
func (m *mcSource) AddFrame(f int) error       { return m.ch.Add(f) }

// TestMCMultiplexerConcurrentNext is the master-channel equivalent.
func TestMCMultiplexerConcurrentNext(t *testing.T) {
	const sources = 3
	const perSource = 200

	mux := sdl.NewMCMultiplexer[int]()
	for s := range sources {
		ch := sdl.NewChannel[int](uint8(s), perSource)
		for n := range perSource {
			if err := ch.Add(s*perSource + n); err != nil {
				t.Fatal(err)
			}
		}
		mux.Add(&mcSource{scid: uint16(s), ch: ch}, s+1)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := make(map[int]int)

	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				frame, err := mux.Next()
				if err != nil {
					return
				}
				mu.Lock()
				seen[frame]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(seen) != sources*perSource {
		t.Errorf("got %d distinct frames, want %d", len(seen), sources*perSource)
	}
	for frame, count := range seen {
		if count != 1 {
			t.Errorf("frame %d delivered %d times, want once", frame, count)
		}
	}
}

// TestGapCounterConcurrentTrack drives one GapCounter from many goroutines,
// each on its own channel. GapCounter was the one stateful type in the
// package without a mutex, so the maps raced. Run with -race.
func TestGapCounterConcurrentTrack(t *testing.T) {
	g := sdl.NewGapCounter[uint8](0xFF)

	var wg sync.WaitGroup
	for ch := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range 300 {
				g.Track(uint8(ch), uint8(n))
				_ = g.LastGap()
			}
		}()
	}
	wg.Wait()

	// Each channel counted 0..299 with no loss; a fresh count on any of them
	// must still line up with what that channel expects.
	for ch := range 8 {
		if gap := g.Track(uint8(ch), 300&0xFF); gap != 0 {
			t.Errorf("channel %d gave gap %d after clean sequence, want 0", ch, gap)
		}
	}
}

// TestServiceManagerConcurrentRegistration races registration against lookup,
// which previously wrote the virtualServices and masterChannels maps unlocked.
func TestServiceManagerConcurrentRegistration(t *testing.T) {
	manager := sdl.NewServiceManager[int, int]()

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for vcid := range 32 {
			manager.RegisterVirtualService(uint8(vcid), 0, &noopService{})
		}
	}()
	go func() {
		defer wg.Done()
		for scid := range 32 {
			ch := sdl.NewChannel[int](0, 4)
			manager.RegisterMasterChannel(uint16(scid), &mcSource{scid: uint16(scid), ch: ch})
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 200 {
			_ = manager.HasPendingFramesInMasterChannel(uint16(i % 32))
			_, _ = manager.ReceiveData(uint8(i%32), 0)
		}
	}()

	wg.Wait()
}

// TestMasterChannelSetConcurrentAddAndRoute drives one MasterChannelSet from
// many goroutines, some registering fresh master channels while others route
// frames to and look up channels already registered.
//
// Before the mutex, Add wrote the channels map with no lock while Get, Route,
// and Lowest read it unlocked, so registering a channel while another
// goroutine routed a frame raced: "fatal error: concurrent map read and map
// write", which no recover catches. Run with -race.
func TestMasterChannelSetConcurrentAddAndRoute(t *testing.T) {
	const preRegistered = 8
	const added = 32

	set := sdl.NewMasterChannelSet[int, *mcSource]()
	for s := range preRegistered {
		ch := sdl.NewChannel[int](0, 64)
		set.Add(&mcSource{scid: uint16(s), ch: ch}, 1)
	}

	var wg sync.WaitGroup
	wg.Add(3)

	// Registers fresh master channels, disjoint from the pre-registered and
	// routed SCIDs.
	go func() {
		defer wg.Done()
		for s := range added {
			ch := sdl.NewChannel[int](0, 4)
			set.Add(&mcSource{scid: uint16(preRegistered + s), ch: ch}, 1)
		}
	}()

	// Routes frames to the pre-registered channels.
	go func() {
		defer wg.Done()
		for i := range 500 {
			scid := uint16(i % preRegistered)
			if err := set.Route(scid, i); err != nil {
				t.Errorf("Route(%d): %v", scid, err)
			}
		}
	}()

	// Looks channels up while the other two goroutines add and route.
	go func() {
		defer wg.Done()
		for i := range 500 {
			_, _ = set.Get(uint16(i % preRegistered))
			_, _ = set.Lowest()
			_ = set.Len()
		}
	}()

	wg.Wait()
}

type noopService struct{}

func (noopService) Send([]byte) error        { return nil }
func (noopService) Receive() ([]byte, error) { return nil, nil }
func (noopService) Flush() error             { return nil }
