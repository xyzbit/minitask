package worker

import "github.com/xyzbit/minitaskx/core/model"

func generateInstanceMetadata() (map[string]string, error) {
	metadata := make(map[string]string)

	workerDesc := generateWorkerDesc()
	for k, v := range workerDesc {
		metadata[k] = v
	}

	ru, err := model.GenerateResourceUsage()
	if err != nil {
		return nil, err
	}
	for k, v := range ru {
		metadata[k] = v
	}

	staint, err := model.GenerateStaint(ru)
	if err != nil {
		return nil, err
	}
	for k, v := range staint {
		metadata[k] = v
	}

	return metadata, nil
}

// 获取节点描述
func generateWorkerDesc() map[string]string {
	// TODO name, createtime ...
	return nil
}

func LoadWorkerDesc(metadata map[string]string) map[string]string {
	// TODO name, createtime ...
	return nil
}
