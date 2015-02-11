package main

import (
	"fmt"
	"time"

	"github.com/therealbill/libredis/client"
)

type SentinelStore struct {
	SentinelHosts []string
	Master        *client.Redis
	Slaves        []*client.Redis
	UseSentinel   bool
	RedisAuth     string
	PodName       string
	MasterAddress string
}

type DataStore interface {
	ConnectMaster() error
	StoreEventEntry(instance, event string, value int) error
	GetInstanceEvent(instance, event string) (int, error)
}

func (d *SentinelStore) ConnectSentinel() (error, bool) {
	if !d.UseSentinel {
		return nil, false
	}
	haveconn := false
	for _, addr := range d.SentinelHosts {
		conn, err := client.DialWithConfig(&client.DialConfig{Address: addr})
		if err != nil {
			continue
		} else {
			master, err := conn.SentinelGetMaster(d.PodName)
			if err == nil {
				logger.Info(fmt.Sprintf("%+v", master))
				return nil, true
			}
		}
	}
	return nil, haveconn
}

func (d *SentinelStore) testMasterConn() bool {
	err := d.Master.Ping()
	if err != nil {
		return true
	}
	return false
}

func (d *SentinelStore) ConnectMaster() (master *client.Redis, err error) {
	if d.Master == nil {
		err = fmt.Errorf("No master has been pulled from Sentinels")
		return
	}
	if d.UseSentinel {
		if d.Master == nil {
			d.ConnectSentinel()
		}

		if d.testMasterConn() {
			return d.Master, nil
		}
	}
	return d.Master, nil
}

func (d *SentinelStore) StoreEventEntry(instance, event, value string) error {
	conn, err := d.ConnectMaster()
	if err != nil {
		return err
	}
	keyname := "_latency:i" + instance + ":" + event
	now := time.Now().Unix()
	res, err := conn.ZAdd(keyname, float64(now), value)
	fmt.Printf("%+v\n", res)
	return err
}

func (d *SentinelStore) GetInstanceEvents(instance, event string) (results []int, err error) {
	conn, err := d.ConnectMaster()
	if err != nil {
		return results, err
	}
	keyname := "_latency:i" + instance + ":" + event
	res, err := conn.ZRange(keyname, 0, -1, true)
	fmt.Printf("%+v\n", res)
	return results, err
}
