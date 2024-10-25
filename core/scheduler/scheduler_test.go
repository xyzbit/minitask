package scheduler

import (
	"testing"

	"github.com/xyzbit/minitaskx/core/discover"
	"github.com/xyzbit/minitaskx/core/model"
)

func TestSelectWorkerByResources(t *testing.T) {
	// l := zap.New(
	// 	zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
	// 		zapcore.AddSync(os.Stdout),
	// 		zap.DebugLevel),
	// 	zap.AddCaller(),
	// 	zap.AddCallerSkip(2),
	// 	zap.AddStacktrace(zapcore.ErrorLevel),
	// )
	// log.ReplaceGlobal(log.NewLoggerByzap(l.Sugar()))
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
				{InstanceId: "1", Metadata: map[string]string{model.CpuUsageKey: "32", model.MemUsageKey: "90"}},
				{InstanceId: "2", Metadata: map[string]string{model.CpuUsageKey: "16", model.MemUsageKey: "80"}},
			}},
			want: discover.Instance{InstanceId: "2"},
		},
		{
			name: "结合机器、应用资源使用率考虑",
			args: args{workers: []discover.Instance{
				{InstanceId: "1", Metadata: map[string]string{model.CpuUsageKey: "32", model.MemUsageKey: "32.67", model.GoGoroutineKey: "100", model.GoGcPauseKey: "100", model.GoGcCountKey: "10"}},
				{InstanceId: "2", Metadata: map[string]string{model.CpuUsageKey: "32", model.MemUsageKey: "32.10", model.GoGoroutineKey: "1000", model.GoGcPauseKey: "100", model.GoGcCountKey: "10"}},
			}},
			want: discover.Instance{InstanceId: "1"},
		},
		{
			name: "结合机器、应用资源使用率考虑",
			args: args{workers: []discover.Instance{
				{InstanceId: "1", Metadata: map[string]string{model.CpuUsageKey: "32", model.MemUsageKey: "32.67", model.GoGoroutineKey: "5", model.GoGcPauseKey: "100", model.GoGcCountKey: "10"}},
				{InstanceId: "2", Metadata: map[string]string{model.CpuUsageKey: "32", model.MemUsageKey: "32.10", model.GoGoroutineKey: "5", model.GoGcPauseKey: "1000", model.GoGcCountKey: "10"}},
			}},
			want: discover.Instance{InstanceId: "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selectedWorker := priorityWorker(tt.args.workers)
			if selectedWorker.InstanceId != tt.want.InstanceId {
				t.Errorf("selectWorkerByResources() = %v, want %v", selectedWorker, tt.want)
			}
		})
	}
}
