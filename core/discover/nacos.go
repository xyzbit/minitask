package discover

import (
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/samber/lo"
)

type nacosDiscover struct {
	serviceName string
	groupName   string
	clusterName string
	nacosClient naming_client.INamingClient
}

type NacosConfig struct {
	IpAddr      string
	Port        uint64
	ServiceName string
	GroupName   string
	ClusterName string
	LogLevel    string
}

func NewNacosDiscover(cfg NacosConfig) (Interface, error) {
	serverConfigs := []constant.ServerConfig{
		{
			IpAddr: cfg.IpAddr,
			Port:   cfg.Port,
		},
	}
	clientConfig := constant.ClientConfig{
		NamespaceId:         "", // 命名空间ID
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		LogLevel:            "debug",
	}

	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, err
	}

	return &nacosDiscover{
		serviceName: cfg.ServiceName,
		groupName:   cfg.GroupName,
		clusterName: cfg.ClusterName,
		nacosClient: namingClient,
	}, nil
}

func (s *nacosDiscover) GetAvailableInstances() ([]Instance, error) {
	availableWorkers, err := s.nacosClient.SelectInstances(vo.SelectInstancesParam{
		ServiceName: s.serviceName,
		GroupName:   s.groupName,
		Clusters:    []string{s.clusterName},
		HealthyOnly: true,
	})
	if err != nil {
		return nil, err
	}

	return lo.Map(availableWorkers, func(item model.Instance, _ int) Instance {
		return Instance{
			Enable:     item.Enable,
			Healthy:    item.Healthy,
			InstanceId: item.InstanceId,
			Ip:         item.Ip,
			Metadata:   item.Metadata,
			Port:       item.Port,
		}
	}), nil
}

func (s *nacosDiscover) Subscribe(callback func(services []Instance, err error)) error {
	return s.nacosClient.Subscribe(&vo.SubscribeParam{
		ServiceName: s.serviceName,
		GroupName:   s.groupName,
		Clusters:    []string{s.clusterName},
		SubscribeCallback: func(services []model.SubscribeService, err error) {
			svcs := lo.Map(services, func(item model.SubscribeService, _ int) Instance {
				return Instance{
					Enable:     item.Enable,
					Healthy:    item.Healthy,
					InstanceId: item.InstanceId,
					Ip:         item.Ip,
					Metadata:   item.Metadata,
					Port:       item.Port,
				}
			})
			callback(svcs, err)
		},
	})
}

func (s *nacosDiscover) Register(i Instance) (bool, error) {
	return s.nacosClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          i.Ip,
		Port:        uint64(i.Port),
		ServiceName: s.serviceName,
		GroupName:   s.groupName,
		ClusterName: s.clusterName,
		Weight:      1,
		Enable:      i.Enable,
		Healthy:     i.Healthy,
		Ephemeral:   true,
		Metadata:    i.Metadata,
	})
}

func (s *nacosDiscover) UnRegister(i Instance) (bool, error) {
	return s.nacosClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          i.Ip,
		Port:        uint64(i.Port),
		ServiceName: s.serviceName,
		GroupName:   s.groupName,
	})
}
