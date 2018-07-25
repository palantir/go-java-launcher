// Copyright 2016 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Adapted from github.com/jonboulle/clockwork, which is also published under
// Apache License, Version 2.0.

package time

import (
	"sync"
	"time"
)

// Clock provides an interface that packages can use instead of directly
// using the time module, so that chronology-related behavior can be tested
type Clock interface {
	NewTimer(d time.Duration) Timer
	NewTicker(d time.Duration) Ticker
	Sleep(d time.Duration)
	Now() time.Time
}

type Timer interface {
	Chan() <-chan time.Time
	Stop()
}

type Ticker interface {
	Chan() <-chan time.Time
	Stop()
}

// FakeClock provides an interface for a clock which can be
// manually advanced through time
type FakeClock interface {
	Clock
	// Advance advances the FakeClock to a new point in time, ensuring any existing
	// sleepers are notified appropriately before returning
	Advance(d time.Duration)
	// BlockUntil will block until the FakeClock has the given number of
	// sleepers (callers of Sleep or After)
	BlockUntil(n int)
}

// NewRealClock returns a Clock which simply delegates calls to the actual time
// package; it should be used by packages in production.
func NewRealClock() Clock {
	return &realClock{}
}

// NewFakeClock returns a FakeClock implementation which can be
// manually advanced through time for testing. The initial time of the
// FakeClock will be an arbitrary non-zero time.
func NewFakeClock() FakeClock {
	// use a fixture that does not fulfill Time.IsZero()
	return NewFakeClockAt(time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC))
}

// NewFakeClockAt returns a FakeClock initialised at the given time.Time.
func NewFakeClockAt(t time.Time) FakeClock {
	return &fakeClock{
		time: t,
	}
}

// Real clock

type realClock struct{}

type realTimer struct {
	*time.Timer
}

func (rt *realTimer) Chan() <-chan time.Time {
	return rt.C
}

func (rt *realTimer) Stop() {
	rt.Timer.Stop()
}

type realTicker struct {
	*time.Ticker
}

func (rt *realTicker) Chan() <-chan time.Time {
	return rt.C
}

func (rc *realClock) NewTimer(d time.Duration) Timer {
	return &realTimer{time.NewTimer(d)}
}

func (rc *realClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{time.NewTicker(d)}
}

func (rc *realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (rc *realClock) Now() time.Time {
	return time.Now()
}

// Fake clock

type fakeClock struct {
	sleepers []*sleeper
	blockers []*blocker
	time     time.Time

	l sync.RWMutex
}

type fakeTimer struct {
	c       chan time.Time
	fc      *fakeClock
	sleeper *sleeper
}

func (ft *fakeTimer) Chan() <-chan time.Time {
	return ft.c
}

func (ft *fakeTimer) Stop() {
	if ft.sleeper != nil {
		fc := ft.fc
		// Close the channel before locking! That's important
		close(ft.c)
		fc.l.Lock()
		defer fc.l.Unlock()
		// Remove the sleeper (if it exists) and notify any blockers
		newSleepers := make([]*sleeper, 0)
		for _, s := range fc.sleepers {
			if s != ft.sleeper {
				newSleepers = append(newSleepers, s)
			}
		}
		fc.sleepers = newSleepers
		fc.blockers = notifyBlockers(fc.blockers, len(fc.sleepers))
	}
}

// NewTimer mimics time.NewTimer; it waits for the given duration to elapse on the
// fakeClock, then sends the current time on the returned channel.
func (fc *fakeClock) NewTimer(d time.Duration) Timer {
	done, sleeper := fc.newTimerOrTicker(d, false)
	return &fakeTimer{done, fc, sleeper}
}

func (fc *fakeClock) NewTicker(d time.Duration) Ticker {
	done, sleeper := fc.newTimerOrTicker(d, true)
	return &fakeTimer{done, fc, sleeper}
}

func (fc *fakeClock) newTimerOrTicker(d time.Duration, periodic bool) (chan time.Time, *sleeper) {
	fc.l.Lock()
	defer fc.l.Unlock()
	now := fc.time
	done := make(chan time.Time, 0)
	var s *sleeper
	if d.Nanoseconds() == 0 {
		// special case - trigger immediately
		done <- now
	} else {
		// otherwise, add to the set of sleepers
		s = &sleeper{
			until: now.Add(d),
			done:  done,
		}
		if periodic {
			s.period = &d
		}
		fc.sleepers = append(fc.sleepers, s)
		// and notify any blockers
		fc.blockers = notifyBlockers(fc.blockers, len(fc.sleepers))
	}
	return done, s
}

// sleeper represents a caller of After or Sleep
type sleeper struct {
	until  time.Time
	period *time.Duration
	done   chan time.Time
}

// blocker represents a caller of BlockUntil
type blocker struct {
	count int
	ch    chan struct{}
}

// notifyBlockers notifies all the blockers waiting until the
// given number of sleepers are waiting on the fakeClock. It
// returns an updated slice of blockers (i.e. those still waiting)
func notifyBlockers(blockers []*blocker, count int) (newBlockers []*blocker) {
	for _, b := range blockers {
		if b.count == count {
			close(b.ch)
		} else {
			newBlockers = append(newBlockers, b)
		}
	}
	return
}

// Sleep blocks until the given duration has passed on the fakeClock
func (fc *fakeClock) Sleep(d time.Duration) {
	<-fc.NewTimer(d).Chan()
}

// Time returns the current time of the fakeClock
func (fc *fakeClock) Now() time.Time {
	fc.l.RLock()
	t := fc.time
	fc.l.RUnlock()
	return t
}

// Advance time until `end` given the sleepers, recursing to re-evaluate periodic sleepers
func advanceTailRec(end time.Time, sleepers []*sleeper, newSleepers []*sleeper) []*sleeper {
	periodicSleepers := make([]*sleeper, 0)
	for _, s := range sleepers {
		//log.Printf("Examining sleeper at %v\n", s.until)
		if end.Sub(s.until) >= 0 {
			s.done <- s.until
			// Re-schedule if necessary
			if s.period != nil {
				s := &sleeper{
					until:  s.until.Add(*s.period),
					period: s.period,
					done:   s.done,
				}
				//log.Printf("Ran periodic sleeper at %v\n", s.until)
				periodicSleepers = append(periodicSleepers, s)
			} else {
				//log.Printf("FINISHED sleeper at %v\n", s.until)
			}
		} else {
			//log.Printf("Copying over sleeper at %v\n", s.until)
			newSleepers = append(newSleepers, s)
		}
	}
	if len(periodicSleepers) != 0 {
		return advanceTailRec(end, periodicSleepers, newSleepers)
	}
	return newSleepers
}

// Advance advances fakeClock to a new point in time, ensuring channels from any
// previous invocations of After are notified appropriately before returning
func (fc *fakeClock) Advance(d time.Duration) {
	fc.l.Lock()
	defer fc.l.Unlock()
	end := fc.time.Add(d)
	//log.Printf("Advancing from %v -> %v\n", fc.time.String(), end.String())
	newSleepers := make([]*sleeper, 0)
	newSleepers = advanceTailRec(end, fc.sleepers, newSleepers)
	fc.sleepers = newSleepers
	fc.blockers = notifyBlockers(fc.blockers, len(fc.sleepers))
	fc.time = end
}

// BlockUntil will block until the fakeClock has the given number of sleepers
// (callers of Sleep or After)
func (fc *fakeClock) BlockUntil(n int) {
	fc.l.Lock()
	// Fast path: current number of sleepers is what we're looking for
	if len(fc.sleepers) == n {
		fc.l.Unlock()
		return
	}
	// Otherwise, set up a new blocker
	b := &blocker{
		count: n,
		ch:    make(chan struct{}),
	}
	fc.blockers = append(fc.blockers, b)
	fc.l.Unlock()
	<-b.ch
}
