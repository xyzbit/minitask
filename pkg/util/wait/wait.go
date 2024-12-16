package wait

import (
	"errors"
	"math/rand/v2"
	"time"
)

var ErrWaitTimeout = errors.New("timed out waiting for the condition")

// Backoff holds parameters applied to a Backoff function.
type Backoff struct {
	// The initial duration.
	Duration time.Duration
	// Duration is multiplied by factor each iteration, if factor is not zero
	// and the limits imposed by Steps and Cap have not been reached.
	// Should not be negative.
	// The jitter does not contribute to the updates to the duration parameter.
	Factor float64
	// The sleep at each iteration is the duration plus an additional
	// amount chosen uniformly at random from the interval between
	// zero and `jitter*duration`.
	Jitter float64
	// The remaining number of iterations in which the duration
	// parameter may change (but progress can be stopped earlier by
	// hitting the cap). If not positive, the duration is not
	// changed. Used for exponential backoff in combination with
	// Factor and Cap.
	Steps int
	// A limit on revised values of the duration parameter. If a
	// multiplication by the factor parameter would make the duration
	// exceed the cap then the duration is set to the cap and the
	// steps parameter is set to zero.
	Cap time.Duration
}

// Step returns an amount of time to sleep determined by the original
// Duration and Jitter. The backoff is mutated to update its Steps and
// Duration. A nil Backoff always has a zero-duration step.
func (b *Backoff) Step() time.Duration {
	if b == nil {
		return 0
	}
	var nextDuration time.Duration
	nextDuration, b.Duration, b.Steps = delay(b.Steps, b.Duration, b.Cap, b.Factor, b.Jitter)
	return nextDuration
}

// delay implements the core delay algorithm used in this package.
func delay(steps int, duration, cap time.Duration, factor, jitter float64) (_ time.Duration, next time.Duration, nextSteps int) {
	// when steps is non-positive, do not alter the base duration
	if steps < 1 {
		if jitter > 0 {
			return Jitter(duration, jitter), duration, 0
		}
		return duration, duration, 0
	}
	steps--

	// calculate the next step's interval
	if factor != 0 {
		next = time.Duration(float64(duration) * factor)
		if cap > 0 && next > cap {
			next = cap
			steps = 0
		}
	} else {
		next = duration
	}

	// add jitter for this step
	if jitter > 0 {
		duration = Jitter(duration, jitter)
	}

	return duration, next, steps
}

// Jitter returns a time.Duration between duration and duration + maxFactor *
// duration.
//
// This allows clients to avoid converging on periodic behavior. If maxFactor
// is 0.0, a suggested default value will be chosen.
func Jitter(duration time.Duration, maxFactor float64) time.Duration {
	if maxFactor <= 0.0 {
		maxFactor = 1.0
	}
	wait := duration + time.Duration(rand.Float64()*maxFactor*float64(duration))
	return wait
}

// ConditionFunc returns true if the condition is satisfied, or an error
// if the loop should be aborted.
type ConditionFunc func() (done bool, err error)

func ExponentialBackoff(backoff Backoff, condition ConditionFunc) error {
	for backoff.Steps > 0 {
		if ok, err := condition(); err != nil || ok {
			return err
		}
		if backoff.Steps == 1 {
			break
		}
		time.Sleep(backoff.Step())
	}
	return ErrWaitTimeout
}
