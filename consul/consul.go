package consul

import (
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/patrickmn/go-cache"
)

var nodeCache *cache.Cache
var client *api.Client

func init() {
	nodeCache = cache.New(5*time.Minute, 10*time.Minute)
}

func loadNodesFromAllDatacenters() error {
	if client == nil {
		var err error
		client, err = api.NewClient(&api.Config{})
		if err != nil {
			return err
		}
	}

	dcs, err := client.Catalog().Datacenters()
	if err != nil {
		return err
	}

	for _, dc := range dcs {
		nodes, _, err := client.Catalog().Nodes(&api.QueryOptions{
			Datacenter: dc,
		})
		if err != nil {
			return err
		}
		for _, n := range nodes {
			if n == nil {
				continue
			}
			if name, ok := n.Meta["name"]; ok {
				nodeCache.Set(name, n, cache.DefaultExpiration)
			}
		}
	}
	return nil
}

func GetNode(host string) *api.Node {
	if n, found := nodeCache.Get(host); found {
		return n.(*api.Node)
	}
	return nil
}
