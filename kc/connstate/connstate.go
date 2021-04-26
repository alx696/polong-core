package connstate

import (
	"fmt"
	"sync"
)

var sm sync.RWMutex

// 连接状态
var dm = make(map[string]bool)

// Set 设置
func Set(k string, v bool) {
	sm.Lock()
	dm[k] = v
	sm.Unlock()
}

// Get 获取指定
func Get(k string) (bool, error) {
	sm.RLock()
	data, exists := dm[k]
	sm.RUnlock()
	if !exists {
		return false, fmt.Errorf("没有")
	}
	return data, nil
}

// Del 删除
func Del(k string) {
	sm.Lock()
	delete(dm, k)
	sm.Unlock()
}

// TrueKeys 获取所有值为真的键(处于连线状态)
func TrueKeys() []string {
	var array []string
	sm.RLock()
	for k, v := range dm {
		if !v {
			continue
		}

		array = append(array, k)
	}
	sm.RUnlock()
	return array
}
