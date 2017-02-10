package audittap

import "time"

// Injectable time (supports testing)
type Clock interface {
	Now() time.Time
}

type normalClock struct{}

func (c normalClock) Now() time.Time {
	return time.Now()
}

var clock Clock = normalClock{}
