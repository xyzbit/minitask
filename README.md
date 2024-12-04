# 简介
一个简单、优雅、高拓展性的任务调度&管理系统；
支持通过 gorutine、goplugin、docker、k8s cronjob 等方式执行任务，统一的控制任务的生命周期(启动、暂停、终止)。

## 使用场景
当你苦于长任务的各种状态转化控制，任务可靠性保证等问题时，你可以使用 minitaskx 快速启动一个高可用的任务调度&管理系统

## 快速开始
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

## 文档
[系统架构](./docs/architecture.md)