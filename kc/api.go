package kc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	kcconnstate "github.com/alx696/polong-core/kc/connstate"
	kccontact "github.com/alx696/polong-core/kc/contact"
	kcdb "github.com/alx696/polong-core/kc/db"
	kcoption "github.com/alx696/polong-core/kc/option"
	kc_remote_control "github.com/alx696/polong-core/kc/remote_control"
	"github.com/libp2p/go-libp2p-core/peer"
)

// ---------------------------------------------------------
// 以下为公开接口, GetID()获取ID之后才能调用, 记得先要设置我的信息!
// ---------------------------------------------------------

// GetState 获取状态, 返回State
func GetState() []byte {
	peerCount := h.Peerstore().Peers().Len()
	connCount := len(h.Network().Conns())
	jsonBytes, _ := json.Marshal(State{PeerCount: peerCount, ConnCount: connCount})
	return jsonBytes
}

// GetOption 获取设置
func GetOption() string {
	jsonBytes, _ := json.Marshal(kcoption.Get())
	return string(jsonBytes)
}

// SetInfo 设置我的名字和头像
func SetInfo(name, photo string) {
	option := kcoption.Get()
	option.Name = name
	option.Photo = photo
	kcoption.Set(*option)
}

// SetBootstrapArray 设置引导地址
func SetBootstrapArray(arrayText string) error {
	option := kcoption.Get()
	if arrayText == "" {
		option.BootstrapArray = []string{}
	} else {
		option.BootstrapArray = strings.Split(arrayText, ",")
	}

	for _, v := range option.BootstrapArray {
		//检查地址
		_, e := multiaddrToAddrInfo(v)
		if e != nil {
			return fmt.Errorf("破址错误: %s", v)
		}

		go connectBootstrap(v)
	}

	kcoption.Set(*option)
	return nil
}

// SetBlacklistIDArray 设置黑名单
func SetBlacklistIDArray(arrayText string) {
	option := kcoption.Get()
	if arrayText == "" {
		option.BlacklistIDArray = []string{}
	} else {
		option.BlacklistIDArray = strings.Split(arrayText, ",")
	}
	kcoption.Set(*option)

	for _, v := range option.BlacklistIDArray {
		if kccontact.Has(v) {
			DelContact(v)
		}
	}
}

// GetContact 获取联系人Map
func GetContact() string {
	jsonBytes, _ := json.Marshal(kccontact.GetMap())
	return string(jsonBytes)
}

// NewContact 添加(更新)联系人
func NewContact(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("id没设")
	}

	_, e := peer.Decode(id)
	if e != nil {
		return "", fmt.Errorf("id错误")
	}

	if id == h.ID().Pretty() {
		return "", fmt.Errorf("不能是自己")
	}

	// 交换信息
	info, e := exchangeInfo(id)
	if e != nil {
		return "", e
	}

	jsonBytes, _ := json.Marshal(info)

	return string(jsonBytes), nil
}

// DelContact 删除联系人
func DelContact(id string) {
	DeleteChatMessageByPeerID(id)
	kccontact.Del(id)

	// 执行订阅回调
	feedCallback.FeedCallbackOnContactDelete(id)
}

// SetContactNameRemark 设置联系人名字备注
func SetContactNameRemark(id, name string) {
	data := kccontact.Get(id)
	data.NameRemark = name
	kccontact.Set(*data)

	// 执行订阅回调
	jsonBytes, _ := json.Marshal(*data)
	feedCallback.FeedCallbackOnContactUpdate(string(jsonBytes))
}

// SendChatMessageText 发送会话消息文本
func SendChatMessageText(peerID, text string) {
	m := kcdb.ChatMessageInfo{ID: time.Now().UnixNano(), FromPeerID: h.ID().Pretty(), ToPeerID: peerID, Text: text, State: "发送", Read: true}

	go sendChatMessage(&m)
}

// SendChatMessageFile 发送会话消息文件
func SendChatMessageFile(peerID, nameWithoutExtension, extension string, size int64) {
	m := kcdb.ChatMessageInfo{ID: time.Now().UnixNano(), FromPeerID: h.ID().Pretty(), ToPeerID: peerID,
		FileSize: size, FileNameWithoutExtension: nameWithoutExtension, FileExtension: extension, State: "发送", Read: true}

	go sendChatMessage(&m)
}

// FindChatMessage 查询指定节点会话消息
func FindChatMessage(peerID string) (string, error) {
	array, e := kcdb.ChatMessageInfoFind(peerID)
	if e != nil {
		return "", e
	}

	if len(*array) == 0 {
		return "[]", nil
	}

	jsonBytes, _ := json.Marshal(*array)
	return string(jsonBytes), nil
}

// DeleteChatMessageByPeerID 通过节点ID删除会话消息
func DeleteChatMessageByPeerID(peerID string) {
	array, _ := kcdb.ChatMessageInfoFind(peerID)
	for _, v := range *array {
		if v.FileSize > 0 {
			os.Remove(filepath.Join(fileDirectory, fmt.Sprint(v.FileNameWithoutExtension, ".", v.FileExtension)))
		}
	}

	kcdb.ChatMessageInfoDeleteByPeerID(peerID)
}

// DeleteChatMessageByID 通过ID删除会话消息
func DeleteChatMessageByID(id int64) {
	m, e := kcdb.ChatMessageInfoGet(id)
	if e != nil {
		log.Println(e)
		return
	}

	if m.FileSize > 0 {
		os.Remove(filepath.Join(fileDirectory, fmt.Sprint(m.FileNameWithoutExtension, ".", m.FileExtension)))
	}

	kcdb.ChatMessageInfoDeleteByID(id)
}

// GetConnectedPeerIDs 获取连接状态节点ID
func GetConnectedPeerIDs() string {
	ids := kcconnstate.TrueKeys()

	if len(ids) == 0 {
		return "[]"
	}

	jsonBytes, _ := json.Marshal(ids)
	return string(jsonBytes)
}

// GetChatMessageUnReadCount 获取未读会话消息数量(按节点ID统计的Map)
func GetChatMessageUnReadCount() (string, error) {
	dm, e := kcdb.ChatMessageInfoUnReadCount()
	if e != nil {
		return "", e
	}

	jsonBytes, _ := json.Marshal(*dm)
	return string(jsonBytes), nil
}

// SetChatMessageReadByPeerID 通过节点ID设置会话消息已读
func SetChatMessageReadByPeerID(peerID string) {
	kcdb.ChatMessageInfoUpdateRead(peerID, true)
}

// RequestRemoteControl 发起远程控制
func RequestRemoteControl(peerID string) error {
	return requestRemoteControl(peerID)
}

// AllowRemoteControl 允许远程控制(info为空字符时拒绝，否则为允许)
func AllowRemoteControl(info string) {
	kc_remote_control.InfoJson = &info
	kc_remote_control.DataChan = make(chan kc_remote_control.VideoInfo)
}

// SendRemoteControlVideoData 发送远程控制数据
func SendRemoteControlVideoData(presentationTimeUs string, data []byte) {
	kc_remote_control.DataChan <- kc_remote_control.VideoInfo{PresentationTimeUs: presentationTimeUs, Data: data}
}
