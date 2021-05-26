package kc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"

	kcoption "github.com/alx696/polong-core/kc/option"
	"github.com/libp2p/go-libp2p-core/network"
)

// 远程控制消息信息
type RemoteControlMessageInfo struct {
	// 请求，响应，关闭
	Type string `json:"type"`
	Text string `json:"text"`
}

// 远程控制视频信息
type RemoteControlVideoInfo struct {
	PresentationTimeUs int64  `json:"presentation_time_us"`
	Data               []byte `json:"data"`
}

// 处理消息
func remoteControlMessageStreamHandler(s network.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	log.Println("远程控制消息发送节点:", remotePeerID)
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 检查拒绝名单
	if valueInArray(remotePeerID.Pretty(), kcoption.Get().BlacklistIDArray) {
		log.Println("已在拒绝名单中的远程节点:", remotePeerID)
		resultBytes := []byte("拒绝")
		writeTextToReadWriter(rw, &resultBytes)
		return
	}

	// 读取
	requestBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		log.Println("远程控制消息读取出错", e)
		return
	}

	// 解析
	var message RemoteControlMessageInfo
	e = json.Unmarshal(*requestBytes, &message)
	if e != nil {
		log.Println("远程控制消息解析出错", e)
		return
	}

	// 回复
	resultBytes := []byte("收到")
	e = writeTextToReadWriter(rw, &resultBytes)
	if e != nil {
		log.Println("远程控制消息回复出错", e)
		return
	}

	switch message.Type {
	case "请求":
		feedCallback.FeedCallbackOnRemoteControlRequest(remotePeerID.Pretty())
	case "响应":
		feedCallback.FeedCallbackOnRemoteControlResponse(message.Text)
	case "关闭":
		feedCallback.FeedCallbackOnRemoteControlClose()
	default:
		log.Println("远程控制消息遇到不支持类型", message.Type)
	}
}

// 发送消息
func remoteControlMessageSend(id string, message RemoteControlMessageInfo) error {
	s, e := createStream(id, protocolIDRemoteControlMessage)
	if e != nil {
		return e
	}
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 写入
	data, _ := json.Marshal(message)
	e = writeTextToReadWriter(rw, &data)
	if e != nil {
		return e
	}

	// 接收
	resultBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		return e
	}
	result := string(*resultBytes)

	// 检查异常状态
	if result == "拒绝" {
		return fmt.Errorf("拒绝")
	}

	return nil
}

// 处理视频
func remoteControlVideoStreamHandler(s network.Stream) {
	// remotePeerID := s.Conn().RemotePeer()
	// log.Println("远程控制视频发送节点:", remotePeerID)
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 读取元数据
	metadataData := make([]byte, 128)
	rn, e := rw.Read(metadataData)
	if e != nil {
		if e == io.EOF {
			log.Println("远程控制接收视频元数据完毕")
			return
		} else {
			log.Println("远程控制接收视频元数据出错", e)
			return
		}
	}
	if rn != len(metadataData) {
		log.Println("远程控制读取视频元数据失败: 需要128位，实际读取", rn)
		return
	}

	// 解析元数据
	presentationTimeUs, size, e := fromVideoMetadata(metadataData)
	if e != nil {
		log.Println("远程控制解析视频元数据失败: ", e)
		return
	}

	// 读取视频数据
	var videoData []byte
	videoDataBuffer := make([]byte, size)
	for int64(len(videoData)) < size {
		rn, e := rw.Read(videoDataBuffer)
		if e != nil {
			log.Println("远程控制读取视频数据失败: ", e)
			return
		}
		videoData = append(videoData, videoDataBuffer[:rn]...)
	}

	// log.Println("读取视频数据", presentationTimeUs, len(videoData))
	feedCallback.FeedCallbackOnRemoteControlVideo(presentationTimeUs, videoData)
}

// 发送视频
func remoteControlVideoSend(id string, data RemoteControlVideoInfo) error {
	s, e := createStream(id, protocolIDRemoteControlVideo)
	if e != nil {
		return e
	}
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 封装（头部增加时序长度信息）
	videoMetadata := toVideoMetadata(data.PresentationTimeUs, int64(len(data.Data)))
	var videoData []byte
	videoData = append(videoData, videoMetadata...)
	videoData = append(videoData, data.Data...)

	// 发送
	_, e = rw.Write(videoData)
	if e != nil {
		log.Println("远程控制发送视频失败", e)
		return e
	}
	rw.Flush()

	return nil
}
