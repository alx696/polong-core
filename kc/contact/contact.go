package contact

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

// Contact 联系人
type Contact struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Photo      string `json:"photo"` //[可选]
	NameRemark string `json:"nameRemark"`
}

var sm sync.RWMutex
var dm map[string]Contact
var path string

// 保存
func save() error {
	jsonBytes, e := json.Marshal(dm)
	if e != nil {
		return e
	}
	e = ioutil.WriteFile(path, jsonBytes, os.ModePerm)
	if e != nil {
		return e
	}
	return nil
}

// New 创建存储
func New(filePath string) error {
	dm = make(map[string]Contact)
	path = filePath

	_, e := os.Stat(path)
	if e != nil {
		return save()
	}

	jsonBytes, e := ioutil.ReadFile(path)
	if e != nil {
		return e
	}
	e = json.Unmarshal(jsonBytes, &dm)
	if e != nil {
		return e
	}

	return nil
}

// Set 设置
func Set(data Contact) {
	sm.Lock()
	dm[data.ID] = data
	sm.Unlock()
	save()
}

// Del 删除
func Del(id string) {
	sm.Lock()
	delete(dm, id)
	sm.Unlock()
	save()
}

// Has 是否存在
func Has(id string) bool {
	sm.RLock()
	_, exists := dm[id]
	sm.RUnlock()
	return exists
}

// Get 获取指定(返回的是副本)
func Get(id string) *Contact {
	var dataClone Contact
	sm.RLock()
	data, exists := dm[id]
	if exists {
		dataClone = data
	}
	sm.RUnlock()
	return &dataClone
}

// Keys 获取所有ID(返回的是副本)
func Keys() *[]string {
	var array []string
	sm.RLock()
	for k := range dm {
		array = append(array, k)
	}
	sm.RUnlock()
	return &array
}

// Values 获取所有信息(返回的是副本)
func Values() *[]Contact {
	var array []Contact
	sm.RLock()
	for _, v := range dm {
		vClone := v
		array = append(array, vClone)
	}
	sm.RUnlock()
	return &array
}

// GetMap 获取Map(返回的是副本)
func GetMap() *map[string]Contact {
	sm.RLock()
	data := dm
	sm.RUnlock()
	return &data
}
