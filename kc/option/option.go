package option

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

// Option 选项
type Option struct {
	Name             string   `json:"name"`
	Photo            string   `json:"photo"`
	BootstrapArray   []string `json:"bootstrap_array"`    //引导地址
	BlacklistIDArray []string `json:"blacklist_id_array"` //黑名单节点ID
}

var sm sync.RWMutex
var data Option
var path string

// 保存联系人
func save() error {
	jsonBytes, e := json.Marshal(data)
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
	data = Option{BootstrapArray: []string{}, BlacklistIDArray: []string{}}
	path = filePath

	_, e := os.Stat(path)
	if e != nil {
		return save()
	}

	jsonBytes, e := ioutil.ReadFile(path)
	if e != nil {
		return e
	}
	e = json.Unmarshal(jsonBytes, &data)
	if e != nil {
		return e
	}

	return nil
}

// Get 获取指定(返回的是副本)
func Get() *Option {
	sm.RLock()
	dataClone := data
	sm.RUnlock()
	return &dataClone
}

// Set 设置
func Set(newData Option) {
	dataClone := newData
	sm.Lock()
	data = dataClone
	sm.Unlock()
	save()
}
