package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xyzbit/minitaskx/contrib/discover/nacos"
	"github.com/xyzbit/minitaskx/contrib/election/mysql"
	repo "github.com/xyzbit/minitaskx/contrib/taskrepo/mysql"
	"github.com/xyzbit/minitaskx/core/scheduler"
	"github.com/xyzbit/minitaskx/example"
)

var port string

func init() {
	flag.StringVar(&port, "port", "", "scheduler endpoint")
	flag.Parse()
}

func main() {
	nacosDiscover, err := nacos.NewNacosDiscover(nacos.NacosConfig{
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
	taskRepo := repo.NewTaskRepo(example.NewGormDB())
	if err != nil {
		log.Fatalf("创建 Gorm 数据库失败: %v", err)
	}
	_ = example.NewLogger()

	s, err := scheduler.NewScheduler(
		mysql.NewLeaderElector(port, example.NewGormDB()),
		nacosDiscover,
		taskRepo,
	)
	if err != nil {
		log.Fatalf("创建 Scheduler 失败: %v", err)
	}

	if err := s.Run(); err != nil {
		log.Fatalf("启动 Scheduler 失败: %v", err)
	}

	// 启动 gin http 服务

	engine := gin.New()
	httpServer := s.HttpServer()
	engine.POST("api/v1/scheduler/tasks/create", httpServer.CreateTask)
	engine.POST("api/v1/scheduler/tasks/operate", httpServer.OperateTask)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:    net.JoinHostPort("", port),
		Handler: engine,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动服务失败: %v", err)
		}
	}()

	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("服务器关闭失败: %v", err)
	}

	log.Println("服务器已退出")
}
