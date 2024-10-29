# minitask
A simple and graceful task scheduling system

## 快速开始
参考 example 目录下的例子
1. 启动依赖(docker 安装 mysql、nacos)
```shell
   make init
```
3. 启动 minitask worker
```shell
   port=9090 make worker
``` 
4. 启动 minitask scheduler
```shell
port=8080 make scheduler
   ``` 
5. 创建任务
```shell
   make test_create
``` 
