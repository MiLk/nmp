package consul

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/patrickmn/go-cache"
)

var nodeCache *cache.Cache

func init() {
	nodeCache = cache.New(5*time.Minute, 10*time.Minute)
	err := loadEverything()
	if err != nil {
		fmt.Println(err)
	}
}

func loadEverything() error {
	client, err := api.NewClient(&api.Config{})
	if err != nil {
		return err
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
			if n != nil {
				nodeCache.Set(n.Node, n, cache.DefaultExpiration)
			}
		}
	}
	return nil
}

func GetNode(host string) (*api.Node, error) {
	n, found := nodeCache.Get(host)
	if found {
		return n.(*api.Node), nil
	}

	client, err := api.NewClient(&api.Config{})
	if err != nil {
		return nil, err
	}

	dcs, err := client.Catalog().Datacenters()
	if err != nil {
		return nil, err
	}

	var node *api.CatalogNode
	for _, dc := range dcs {
		var err error
		if node, _, err = client.Catalog().Node(host, &api.QueryOptions{
			Datacenter: dc,
		}); err != nil {
			return nil, err
		}
		if node != nil {
			break
		}
	}

	if node == nil {
		return nil, nil
	}

	nodeCache.Set(host, node.Node, cache.DefaultExpiration)

	return node.Node, nil
}
