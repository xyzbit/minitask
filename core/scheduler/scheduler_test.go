package scheduler

import (
	"testing"

	"github.com/xyzbit/minitaskx/core/discover"
)

func TestSelectWorkerByResources(t *testing.T) {
	type args struct {
		workers []discover.Instance
	}
	tests := []struct {
		name    string
		args    args
		want    discover.Instance
		wantErr bool
	}{
		{
			name: "数据异常缺省情况，选择第一个 instance",
			args: args{workers: []discover.Instance{
				{InstanceId: "1", Metadata: map[string]string{}},
				{InstanceId: "2", Metadata: map[string]string{}},
			}},
			want: discover.Instance{InstanceId: "1"},
		},
		{
			name: "选择机器资源使用率低的 instance",
			args: args{workers: []discover.Instance{
				{InstanceId: "1", Metadata: map[string]string{"cpu_usage": "32", "mem_usage": "90"}},
				{InstanceId: "2", Metadata: map[string]string{"cpu_usage": "16", "mem_usage": "80"}},
			}},
			want: discover.Instance{InstanceId: "2"},
		},
		{
			name: "机器资源使用率相差不大，选择应用资源使用率低的 instance",
			args: args{workers: []discover.Instance{
				{InstanceId: "1", Metadata: map[string]string{"cpu_usage": "32", "mem_usage": "32.67", "go_goroutine": "5", "go_gc_pause": "100", "go_gc_count": "10"}},
				{InstanceId: "2", Metadata: map[string]string{"cpu_usage": "32", "mem_usage": "32.10", "go_goroutine": "10", "go_gc_pause": "100", "go_gc_count": "10"}},
			}},
			want: discover.Instance{InstanceId: "2"},
		},
		{
			name: "机器资源使用率相差不大，选择应用资源使用率低的 instance",
			args: args{workers: []discover.Instance{
				{InstanceId: "1", Metadata: map[string]string{"cpu_usage": "32", "mem_usage": "32.67", "go_goroutine": "4", "go_gc_pause": "1000", "go_gc_count": "10"}},
				{InstanceId: "2", Metadata: map[string]string{"cpu_usage": "32", "mem_usage": "32.10", "go_goroutine": "5", "go_gc_pause": "100", "go_gc_count": "10"}},
			}},
			want: discover.Instance{InstanceId: "2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selectedWorker, err := selectWorkerByResources(tt.args.workers)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectWorkerByResources() error = %v, wantErr %v", err, tt.wantErr)
			}
			if selectedWorker.InstanceId != tt.want.InstanceId {
				t.Errorf("selectWorkerByResources() = %v, want %v", selectedWorker, tt.want)
			}
		})
	}
}
