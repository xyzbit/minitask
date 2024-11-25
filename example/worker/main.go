package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xyzbit/minitaskx/core/discover"
	"github.com/xyzbit/minitaskx/core/executor"
	"github.com/xyzbit/minitaskx/core/taskrepo/mysql"
	"github.com/xyzbit/minitaskx/core/worker"
	"github.com/xyzbit/minitaskx/example"
	"github.com/xyzbit/minitaskx/pkg"
	"go.uber.org/zap/zapcore"
)

var (
	port int
	id   string
)

func init() {
	executor.RegisteFactory("simple", NewExecutor)

	flag.StringVar(&id, "id", "", "worker id, if empty, will be auto set to discover instance id")
	flag.IntVar(&port, "port", 0, "worker port")
	flag.Parse()
}

func main() {
	nacosDiscover, err := discover.NewNacosDiscover(discover.NacosConfig{
		IpAddr:      "localhost",
		Port:        8848,
		ServiceName: "example-workers",
		GroupName:   "default",
		ClusterName: "default",
		LogLevel:    "debug",
	})
	if err != nil {
		log.Fatalf("创建 Nacos 客户端失败: %v", err)
	}

	taskrepo := mysql.NewTaskRepo(example.NewGormDB())

	ip, err := pkg.GlobalUnicastIPString()
	if err != nil {
		panic(err)
	}

	var field zapcore.Field
	if id == "" {
		field = zapcore.Field{Key: "worker_id", String: fmt.Sprintf("%s:%d", ip, port), Type: zapcore.StringType}
	} else {
		field = zapcore.Field{Key: "worker_id", String: id, Type: zapcore.StringType}
	}
	logger := example.NewLogger(field)
	worker := worker.NewWorker(
		id, ip, port, nacosDiscover, taskrepo,
		worker.WithLogger(logger), worker.WithShutdownTimeout(15*time.Second),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-quit
		cancel()
	}()

	if err := worker.Run(ctx); err != nil {
		log.Fatalf("启动 Worker 失败: %v", err)
	}
}
