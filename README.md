# 简介

一个简单、优雅、高拓展性的任务调度&管理系统；
支持通过 gorutine、goplugin、docker、k8s cronjob 等方式执行任务，统一的控制任务的生命周期(启动、暂停、终止)。

## 使用场景

当你苦于长任务的各种状态转化控制，任务可靠性保证等问题时，你可以使用 minitaskx 快速启动一个高可用的任务调度&管理系统

## **快速开始**

参考 example 目录下的例子

1. 下载项目并进入 example 目录
```SQL
git clone git@github.com:xyzbit/minitaskx.git
cd ./minitaskx/example
```

2. 启动依赖(需要提前安装 docker)
```Shell
make init
```

3. 启动 minitaskx worker
```Shell
port=9090 make worker
```

4. 启动 minitaskx scheduler
```Shell
port=8080 make scheduler
```

5. 测试任务操作
```Shell
 make test_create
```

  

## 系统架构

![[./docs/architecture.png]]

### 调度器 Scheduler

Scheduler 有两个身份 candidate 和 leader，所有的节点都可接受外部请求，但是只有 leader 节点才能分配任务

### 工作者 Worker

Worker 是任务执行程序，它 会运行 scheduler 分配给它的任务，启动并维护不同 Executors 的生命周期。

它由如下组件构成：

**Watcher** 会通过`ListAndwatch`机制获取到系统任务的实际状态并缓存在内存中，并发送一个同步事件到 `Task Queue`**；**(在启动时会List一次、运行过程中定时List强制刷新缓存)

**Event Queue** 会对任务事件进行去重复，同时可以支持拓展多个任务队列，事件会按照任务标识分片到不同的队列；

**Syncer** 是一个控制循环，会不断比较 `Diff(want task,real task)`, 并将 实际任务状态同步到Executor（real task 会去 `Watcher` 的缓存中获取）

>其中 `Syncer` 是流程的核心逻辑，有了它实际上就能够满足功能需求，然而频繁地调用执行器的接口来获取实际状态并进行比对，会致使系统稳定性下降；`Watcher、Event Queue` 的设计从根本上来说是通过事件通知与缓存的方式来减少执行器接口的调用

  
## 状态流转

![[./docs/task_status.png]]

图中带编号的表示可以人工操作， 分别为 Create：创建任务、Pause：暂停任务、Resume：恢复任务、Exit：退出任务；图中虚线表示任务状态和执行器状态的映射关系；

执行器的状态通过调度系统设置的期望状态保持，也就是上文提到的`Diff(want task,real task)`逻辑：

![[./docs/task_status_option.png]]