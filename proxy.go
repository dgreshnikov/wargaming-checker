package main

import (
	"bufio"
	"errors"
	"net/url"
	"os"
	"sync/atomic"
)

type ProxyRotator struct {
	proxyURLs []*url.URL
	index     uint32
}

func (r *ProxyRotator) GetProxy() *url.URL {
	index := atomic.AddUint32(&r.index, 1) - 1
	u := r.proxyURLs[index%uint32(len(r.proxyURLs))]
	return u
}

func NewProxyRotator(proxies ...string) (*ProxyRotator, error) {
	if len(proxies) < 1 {
		return nil, errors.New("proxy list is empty")
	}

	urls := make([]*url.URL, len(proxies))
	for i, u := range proxies {
		parsedU, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		urls[i] = parsedU
	}
	return &ProxyRotator{urls, 0}, nil
}

func readProxies(path string) ([]string, error) {
	proxiesFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer proxiesFile.Close()

	var proxies []string
	scanner := bufio.NewScanner(proxiesFile)
	for scanner.Scan() {
		proxies = append(proxies, scanner.Text())
	}
	return proxies, nil
}
