package health

import (
	"advanced-lb/balancer"
	"log"
	"net"
	"net/url"
	"time"
)

func StartHealthCheck(getLB func() balancer.LoadBalancer, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			log.Println("Running Health Checks...")
			lb := getLB()
			backends := lb.GetBackends()
			for _, b := range backends {
				alive := isBackendAlive(b.URL)
				lb.UpdateBackendStatus(b.URL, alive)
				status := "UP"
				if !alive {
					status = "DOWN"
				}
				log.Printf("%s [%s]", b.URL, status)
			}
		}
	}()
}

func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
