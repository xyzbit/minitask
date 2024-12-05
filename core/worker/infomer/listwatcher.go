package infomer

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

type Lister interface {
	List(ctx context.Context) ([]*model.Task, error)
}

// Watcher is any object that knows how to start a watch on a resource.
type Watcher interface {
	ResultChan() <-chan Event
}

type ListAndWatcher interface {
	Lister
	Watcher
}

type Event struct {
	Type EventType
	Task *model.Task
}

// EventType defines the possible types of events.
type EventType string

const (
	Added   EventType = "Added"
	Updated EventType = "Updated"
	Deleted EventType = "Deleted"
)
