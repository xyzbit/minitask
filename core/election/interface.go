package election

import "time"

type LeaderElection struct {
	Anchor         int
	MasterID       string
	Endpoint       string
	LastSeenActive time.Time
}

type Interface interface {
	Leader() (*LeaderElection, error)
	AmILeader(leader *LeaderElection) bool
	AttemptElection()
}
