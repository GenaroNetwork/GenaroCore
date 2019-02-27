package utils

import (
	"encoding/binary"
	"github.com/hashicorp/consul/api"
	"log"
	"time"
)

type ConsulClient struct {
	Client *api.Client
}

// Create a new client
func NewClient(addr string) (*ConsulClient, error) {
	conf := api.DefaultConfig()
	conf.Address = addr
	conf.WaitTime = 10 * time.Second

	client, err := api.NewClient(conf)
	consulClient := ConsulClient{
		client,
	}
	return &consulClient, err
}

// PUT a new KV string pair
func (client *ConsulClient) PutStr(k, v string) {
	p := &api.KVPair{Key: k, Value: []byte(v)}
	_, err := client.Client.KV().Put(p, nil)
	if err != nil {
		panic(err)
	}
}

// GET string key value
func (client *ConsulClient) GetStr(k string) (string, error) {
	pair, _, err := client.Client.KV().Get(k, nil)
	if err != nil {
		return "", err
	}
	if pair == nil {
		return "", nil
	}
	return string(pair.Value), nil
}

// PUT a new KV uint64 pair
func (client *ConsulClient) PutUint64(k string, v uint64) {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	p := &api.KVPair{Key: k, Value: b}
	_, err := client.Client.KV().Put(p, nil)
	if err != nil {
		panic(err)
	}
}

// GET uint64 key value
func (client *ConsulClient) GetUint64(k string) (uint64, error) {
	pair, _, err := client.Client.KV().Get(k, nil)
	if err != nil {
		return 0, err
	}
	if pair == nil {
		return 0, nil
	}
	return binary.BigEndian.Uint64(pair.Value), nil
}

func TryLock(lock *api.Lock, delay int) bool {
	timeout := make(chan bool, 1)
	ch := make(chan struct{}, 1)
	ok := make(chan bool, 1)

	go func() {
		chRet, err := lock.Lock(ch)
		if err != nil && api.ErrLockHeld == err {
			ok <- true
			log.Println("has lock ok")
			return
		}
		if err != nil {
			log.Println(err)
			return
		}
		if chRet != nil {
			ok <- true
			log.Println("lock ok")
		} else {
			log.Println("lock failed")
		}
	}()
	go func() {
		time.Sleep(time.Second * time.Duration(delay))
		timeout <- true
	}()

	select {
	case <-ok:
		log.Println("lock success!")
		return true
	case <-timeout:
		log.Println("lock timeout!")
		ch <- struct{}{}
		return false
	}
}
