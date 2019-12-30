package cache

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"sync"
	"time"
)

const DefaultMemory = 100 * 1024 * 1024

type Cache interface {
	// size 是 个字符 。 持以下参数: 1KB，100KB，1MB，2MB，1GB 等
	SetMaxMemory(size string) bool
	// 设置 个缓存项，并且在expire时间之后过期
	Set(key string, val interface{}, expire time.Duration) // 获取 个值
	Get(key string) (interface{}, bool)
	// 删除 个值
	Del(key string) bool
	// 检测 个值 是否存在
	Exists(key string) bool
	// 清空所有值
	Flush() bool
	// 返回所有的key 多少
	Keys() int64
}

type Value struct {
	Value      interface{}
	ExpireTime *time.Timer
}

type SowhyCache struct {
	Lock      sync.RWMutex
	Iterm     map[string]*Value
	Memory    int64
	CurMemory int64
}

func NewSowhyCache() *SowhyCache {
	return &SowhyCache{
		Lock:      sync.RWMutex{},
		Iterm:     make(map[string]*Value),
		Memory:    DefaultMemory,
		CurMemory: 0,
	}
}

func (c *SowhyCache) SetMaxMemory(size string) bool {
	var s int64
	var si int
	var err error
	switch size[len(size)-2:] {
	case "KB", "kb", "kB", "Kb":
		s = 1024
	case "MB", "mb", "mB", "Mb":
		s = 1024 * 1024
	case "GB", "gb", "gB", "Gb":
		s = 1024 * 1024 * 1024
	default:
		s = 1
	}

	if s != 1 {
		si, err = getMax(size[:len(size)-2])
	} else {
		si, err = getMax(size[:len(size)-1])
	}
	if err != nil {
		log.Println(err)
		return false
	}

	// 当重设MaxMemory的时候，检查设置是否满足当前Cache大小
	if c.CurMemory >= s*int64(si) {
		log.Println("can't reset MaxMemory!")
	}

	c.Memory = s * int64(si)
	return true
}

func getMax(size string) (int, error) {
	return strconv.Atoi(size)
}

func (c *SowhyCache) Set(key string, val interface{}, d time.Duration) {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	// 检查是否超出了MaxMemory
	size := sizeof(reflect.ValueOf(val))
	if (c.Memory - c.CurMemory) < int64(size) {
		log.Println("cache is full,this val can't be saved", ", val: ", val)
		return
	}

	if _, found := c.Iterm[key]; found {
		// 当key重复设置的时候
		log.Printf("this key %s is already existed!\n", key)
		return
	}

	c.Set(key, val, d)
}

func (c *SowhyCache) set(key string, val interface{}, d time.Duration) {
	c.Iterm[key] = &Value{
		Value: val,
	}

	// 判断是否有过期时间
	if d > 0 {
		c.Iterm[key].ExpireTime = time.NewTimer(d)
		go c.run(key)
	}
}

func (c *SowhyCache) run(key string) {
	<-c.Iterm[key].ExpireTime.C
	c.Del(key)
}

func (c *SowhyCache) Get(key string) (interface{}, bool) {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	if val, found := c.Iterm[key]; found {
		return val, true
	}
	err := fmt.Sprintf("there is no key named %s!", key)
	return errors.New(err), false
}

func (c *SowhyCache) Del(key string) bool {
	c.Lock.Lock()
	if _, found := c.Iterm[key]; found {
		delete(c.Iterm, key)
		c.Lock.Unlock()
		return true
	}
	log.Printf("there is no key named %s!", key)
	c.Lock.Unlock()
	return false
}

func (c *SowhyCache) Exists(key string) bool {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	if _, found := c.Iterm[key]; found {
		c.Lock.RUnlock()
		return true
	}
	return false
}

func (c *SowhyCache) Flush() bool {
	c.Lock.Lock()
	for key, _ := range c.Iterm {
		if c.Iterm[key].ExpireTime == nil {
			c.Iterm[key].ExpireTime = time.NewTimer(0)
			c.run(key)
		} else {
			c.Iterm[key].ExpireTime.Reset(0)
		}
	}
	c.Lock.Unlock()
	return true
}

func (c *SowhyCache) Keys() int64 {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	return int64(len(c.Iterm))
}
