package worker

import "github.com/xyzbit/minitaskx/core/model"

func (w *Worker) generateInstanceMetadata() (map[string]string, error) {
	metadata := make(map[string]string)

	workerDesc := w.generateWorkerDesc()
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

	staint, err := model.GenerateStaint(ru, false)
	if err != nil {
		return nil, err
	}
	for k, v := range staint {
		metadata[k] = v
	}

	return metadata, nil
}

// 获取节点描述
func (w *Worker) generateWorkerDesc() map[string]string {
	return map[string]string{
		"worker_id": w.id,
	}
}

func LoadWorkerDesc(metadata map[string]string) map[string]string {
	// TODO name, createtime ...
	return nil
}
