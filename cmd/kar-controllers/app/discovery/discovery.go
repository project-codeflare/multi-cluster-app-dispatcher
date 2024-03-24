package discovery

import (
	"k8s.io/klog/v2"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
)

var GlobalCachedDiscoveryClient discovery.CachedDiscoveryInterface

func InitializeGlobalDiscoveryClient(config *rest.Config) error {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	GlobalCachedDiscoveryClient = memory.NewMemCacheClient(discoveryClient)
	
	go startDiscoveryRefreshTicker()
	
	return nil
}

func RefreshDiscoveryCache() {
	klog.Infof("Invalidating discovery cache")
	GlobalCachedDiscoveryClient.Invalidate()
}

func startDiscoveryRefreshTicker() {
    ticker := time.NewTicker(5 * time.Hour)
    for {
        select {
        case <-ticker.C:
            RefreshDiscoveryCache()
        }
    }
}
