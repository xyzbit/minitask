# 获取项目根路径
PROJECT_ROOT=$(shell git rev-parse --show-toplevel)

# 获取项目路径
PROJECT_PATH=$(shell pwd)

.PHONY: init
init:
	docker run --name minitaskx-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=123456 -d mysql:8.0
	docker run --name minitaskx-nacos -d -p 8848:8848 --network nacos_network -e MODE=standalone nacos/nacos-server

.PHONY: worker
worker:
	go run ${PROJECT_PATH}/worker/*.go -port ${port}

.PHONY: scheduler
scheduler:
	go run ${PROJECT_PATH}/scheduler/*.go -port ${port}

.PHONY: test_create
test_create:
	@echo "创建任务"
	@curl --location --request POST 'http://127.0.0.1:8080/api/v1/scheduler/tasks/create' \
	--header 'Content-Type: application/json' \
	--data-raw '{"biz_id": "1","biz_type": "test","type": "simple","payload": "这是一个测试任务"}'
	