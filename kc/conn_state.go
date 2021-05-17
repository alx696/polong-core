package kc

import (
	"log"
	"time"

	kcconnstate "github.com/alx696/polong-core/kc/connstate"
	kccontact "github.com/alx696/polong-core/kc/contact"
	"github.com/libp2p/go-libp2p-core/peer"
)

// 连接状态重复器信道
var connStateTickerStopChan = make(chan bool, 1)

// 连接状态重复器
var connStateTicker *time.Ticker

// 连接状态重复器任务
func connStateTickerTask() {
	for {
		select {
		case <-connStateTickerStopChan:
			log.Println("连接状态重复器任务结束")
			return
		case <-connStateTicker.C:
			for _, id := range *(kccontact.Keys()) {
				go connStateCheck(id)
			}
		}
	}
}

// 连接状态检查
func connStateCheck(id string) {
	peerID, e := peer.Decode(id)
	if e != nil {
		log.Println("ID错误", id)
		return
	}

	// 获取当前连接状态缓存
	isConnect, stateError := kcconnstate.Get(id)

	// 获取当前连接数量
	connCount := len(h.Network().ConnsToPeer(peerID))

	// 更新缓存
	kcconnstate.Set(id, connCount > 0)

	if connCount == 0 {
		// 当前处于断开状态

		if stateError == nil && isConnect {
			// 如果之前是连接状态, 则为断开
			log.Println("连接断开", id)

			// 订阅回调
			feedCallback.FeedCallbackOnPeerConnectState(id, false)
		}

		// 进行连接
		e = connectDHTPeer(peerID)
		if e != nil {
			// log.Println("连接失败", id, e)
		} else {
			log.Println("连接建立", id)

			// 更新缓存
			kcconnstate.Set(id, true)
			// 订阅回调
			feedCallback.FeedCallbackOnPeerConnectState(id, true)
		}
	} else {
		// 当前处于连接状态

		if stateError != nil || !isConnect {
			// 如果之前(没有状态)是非连接状态, 则为连上
			log.Println("连接建立", id)

			// 订阅回调
			feedCallback.FeedCallbackOnPeerConnectState(id, true)
		}
	}
}
