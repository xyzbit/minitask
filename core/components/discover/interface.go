package discover

type Instance struct {
	InstanceId string            `json:"instanceId"`
	Ip         string            `json:"ip"`
	Port       uint64            `json:"port"`
	Healthy    bool              `json:"healthy"`
	Enable     bool              `json:"enabled"`
	Metadata   map[string]string `json:"metadata"`
}

func (i Instance) ID() string {
	if len(i.Metadata) > 0 && i.Metadata["worker_id"] != "" {
		return i.Metadata["worker_id"]
	}
	return i.InstanceId
}

type Interface interface {
	GetAvailableInstances() ([]Instance, error)
	UpdateInstance(Instance) error
	Subscribe(callback func(services []Instance, err error)) error
	Register(i Instance) (bool, error)
	UnRegister(i Instance) (bool, error)
}
