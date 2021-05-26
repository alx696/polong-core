package kc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	kccontact "github.com/alx696/polong-core/kc/contact"
	kcdb "github.com/alx696/polong-core/kc/db"
	kcoption "github.com/alx696/polong-core/kc/option"
	kc_remote_control "github.com/alx696/polong-core/kc/remote_control"
	"github.com/libp2p/go-libp2p"
	autonat "github.com/libp2p/go-libp2p-autonat"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
	"github.com/libp2p/go-libp2p/p2p/discovery"
)

const (
	// 信息交换协议ID
	protocolIDInfo = "/lilu.red/kc/1/info"
	// 文本消息的协议ID
	protocolIDMessageText = "/lilu.red/kc/1/message/text"
	// 文件消息的协议ID
	protocolIDMessageFile = "/lilu.red/kc/1/message/file"
	// 远程控制的协议ID
	protocolIDRemoteControl = "/lilu.red/kc/1/remote_control"
	// mDNS服务标记
	mDNSServiceTag = "/lilu.red/kc/mdns"
	// 连接保护标记:保持,权重100.
	connProtectTagKeep = "keep"
)

// FeedCallback 订阅回调
type FeedCallback interface {
	// 联系人更新(新增)
	FeedCallbackOnContactUpdate(json string)
	// 联系人删除
	FeedCallbackOnContactDelete(id string)
	// 节点连接状态变化
	FeedCallbackOnPeerConnectState(id string, isConnect bool)
	// 会话消息
	FeedCallbackOnChatMessage(peerID string, chatMessage string)
	// 会话消息状态
	FeedCallbackOnChatMessageState(peerID string, messageID int64, state string)
	// 远程控制收到视频信息
	FeedCallbackOnRemoteControlReceiveVideoInfo(json string)
	// 远程控制收到视频数据
	FeedCallbackOnRemoteControlReceiveVideoData(presentationTimeUs int64, data []byte)
	// 远程控制收到请求
	FeedCallbackOnRemoteControlRequest(peerID string)
}

// State 状态
type State struct {
	PeerCount int `json:"peerCount"` // 节点数量
	ConnCount int `json:"connCount"` // 连接数量
}

// FileInfo 文件信息
type FileInfo struct {
	Size int64 `json:"size"`
	// 名字, 不含类型
	NameWithoutExtension string `json:"nameWithoutExtension"`
	// 类型, 不含"."" 例如"hi.txt"则为"txt"
	Extension string `json:"extension"`
}

var e error
var ctx context.Context
var ctxCancel context.CancelFunc
var idht *dht.IpfsDHT
var h host.Host
var fileDirectory string         // 文件目录
var feedCallback FeedCallback    // 实时推送回调
var ready bool                   // 节点是否就绪标记
var stopChan = make(chan int, 1) //节点是否停止标记

// 处理远程控制
func remoteControlStreamHandler(s network.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	log.Println("发起远程控制节点:", remotePeerID)
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 检查拒绝名单
	if valueInArray(remotePeerID.Pretty(), kcoption.Get().BlacklistIDArray) {
		log.Println("节点已在黑名单中:", remotePeerID)
		resultBytes := []byte("拒绝")
		writeTextToReadWriter(rw, &resultBytes)
		return
	}

	// 等待确认
	feedCallback.FeedCallbackOnRemoteControlRequest(remotePeerID.Pretty())
	for kc_remote_control.InfoJson == nil {
		time.Sleep(time.Millisecond * 500)
	}
	info := *kc_remote_control.InfoJson
	if info == "" {
		log.Println("用户拒绝远程控制:", remotePeerID)
		resultBytes := []byte("拒绝")
		writeTextToReadWriter(rw, &resultBytes)
		return
	}

	// 发送视频信息（宽度，高度）
	resultBytes := []byte(info)
	writeTextToReadWriter(rw, &resultBytes)

	// 发送视频数据
	for {
		select {
		case data := <-kc_remote_control.DataChan:
			// presentationTimeUsBytes := []byte(data.PresentationTimeUs)
			// e = writeTextToReadWriter(rw, &presentationTimeUsBytes)
			// if e != nil {
			// 	log.Println("远程控制发送视频时序失败")
			// 	kc_remote_control.InfoJson = nil
			// 	return
			// }
			// log.Println("远程控制发送视频时序", data.PresentationTimeUs)

			// e = writeDataToReadWriter(rw, &data.Data)
			// if e != nil {
			// 	log.Println("远程控制发送视频数据失败")
			// 	kc_remote_control.InfoJson = nil
			// 	return
			// }
			// log.Println("远程控制发送视频数据", len(data.Data))

			videoMetadata := toVideoMetadata(data.PresentationTimeUs, int64(len(data.Data)))
			var videoData []byte
			videoData = append(videoData, videoMetadata...)
			videoData = append(videoData, data.Data...)

			_, e := rw.Write(videoData)
			if e != nil {
				log.Println("远程控制发送视频数据失败", e)
				return
			}

			rw.Flush()
			// log.Println("远程控制发送视频数据", data.PresentationTimeUs, len(data.Data), len(videoData), wn)
			//-
		case <-kc_remote_control.QuitChan:
			log.Println("远程控制停止发送视频数据")
			return
		}
	}
}

// 发起远程控制
func requestRemoteControl(id string) error {
	s, e := createStream(id, protocolIDRemoteControl)
	if e != nil {
		return e
	}
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 接收首个回复
	resultBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		return e
	}
	result := string(*resultBytes)

	// 检查首个回复
	if result == "拒绝" {
		return fmt.Errorf("拒绝")
	}

	// 反馈视频信息（宽度，高度）
	feedCallback.FeedCallbackOnRemoteControlReceiveVideoInfo(result)

	// 接收视频数据（首条数据必须是CSD）
	for {
		select {
		case <-kc_remote_control.QuitChan:
			log.Println("远程控制停止接收视频数据")
			return nil
		default:
			// videoPresentationTimeUsBytes, e := readTextFromReadWriter(rw)
			// if e != nil {
			// 	return fmt.Errorf("读取视频时序出错: %w", e)
			// }
			// videoPresentationTimeUs := string(*videoPresentationTimeUsBytes)
			// log.Println("读取视频时序", videoPresentationTimeUs)

			// videoData, e := readDataFromReadWriter(rw)
			// if e != nil {
			// 	return fmt.Errorf("读取视频数据出错: %w", e)
			// }
			// log.Println("读取视频数据", len(*videoData))

			metadataData := make([]byte, 128)
			rn, e := rw.Read(metadataData)
			if e != nil {
				if e == io.EOF {
					log.Println("接收视频数据：没有更多数据")
					return nil
				} else {
					return e
				}
			}
			if rn != len(metadataData) {
				return fmt.Errorf("远程控制读取视频元数据失败: 需要128位，实际读取%d", rn)
			}

			presentationTimeUs, size, e := fromVideoMetadata(metadataData)
			if e != nil {
				return fmt.Errorf("远程控制解析视频元数据失败: %w", e)
			}

			var videoData []byte
			videoDataBuffer := make([]byte, size)
			for int64(len(videoData)) < size {
				rn, e := rw.Read(videoDataBuffer)
				if e != nil {
					return fmt.Errorf("远程控制读取视频数据失败: %w", e)
				}
				videoData = append(videoData, videoDataBuffer[:rn]...)
			}

			// log.Println("读取视频数据", presentationTimeUs, len(videoData))
			feedCallback.FeedCallbackOnRemoteControlReceiveVideoData(presentationTimeUs, videoData)
			//-
		}
	}
}

// 处理文件消息
func messageFileStreamHandler(s network.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	log.Println("远程节点文件消息:", remotePeerID)
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

	resultBytes := []byte("继续")
	writeTextToReadWriter(rw, &resultBytes)

	// 读取文件信息
	fileInfoBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		log.Println("读取文件信息出错", e)
		return
	}
	var fileInfo FileInfo
	e = json.Unmarshal(*fileInfoBytes, &fileInfo)
	if e != nil {
		log.Println("解码文件信息出错", e)
		return
	}
	log.Println("收到文件信息内容:", fileInfo)

	// 准备文件路径
	filePath := filepath.Join(fileDirectory, fmt.Sprintf("%s.%s", fileInfo.NameWithoutExtension, fileInfo.Extension))
	_, e = os.Stat(filePath)
	if e == nil {
		fileInfo.NameWithoutExtension = fmt.Sprintf("%s[%d]", fileInfo.NameWithoutExtension, time.Now().Nanosecond())
	}
	filePath = filepath.Join(fileDirectory, fmt.Sprintf("%s.%s", fileInfo.NameWithoutExtension, fileInfo.Extension))

	// 保存消息
	m := kcdb.ChatMessageInfo{ID: time.Now().UnixNano(), FromPeerID: remotePeerID.Pretty(), ToPeerID: h.ID().Pretty(), Text: "", FileSize: fileInfo.Size, FileNameWithoutExtension: fileInfo.NameWithoutExtension, FileExtension: fileInfo.Extension, State: "接收", Read: false}
	e = kcdb.ChatMessageInfoInsert(&m)
	if e != nil {
		log.Println("保存消息时出错", e)
		return
	}

	// 执行订阅回调(说明开始接收文件了)
	jsonBytes, _ := json.Marshal(m)
	feedCallback.FeedCallbackOnChatMessage(m.FromPeerID, string(jsonBytes))

	// 读取文件数据
	f, _ := os.Create(filePath)
	defer f.Close()
	var doneSum int64 //完成长度
	buf := make([]byte, 1048576)
	for {
		var rn int
		rn, e = rw.Read(buf)
		if e != nil {
			if e == io.EOF {
				log.Println("消息文件接收：读取文件时没有更多数据")
			} else {
				log.Println("消息文件接收出错", e)
				// 保存入库
				kcdb.ChatMessageInfoUpdateState(m.ID, "失败")
				// 订阅回调
				feedCallback.FeedCallbackOnChatMessageState(m.FromPeerID, m.ID, fmt.Sprintf(`失败: %s`, e.Error()))
				return
			}
		}

		var wn int
		if rn > 0 {
			wn, e = f.Write(buf[0:rn])
			if e != nil {
				log.Println("消息文件接收出错", e)
				// 保存入库
				kcdb.ChatMessageInfoUpdateState(m.ID, "失败")
				// 订阅回调
				feedCallback.FeedCallbackOnChatMessageState(m.FromPeerID, m.ID, fmt.Sprintf(`失败: %s`, e.Error()))
				return
			}
		}

		// 累加完成长度
		doneSum += int64(wn)
		// 计算百分比
		percentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(doneSum)/float64(fileInfo.Size)), 64)

		// 判断是否完成
		// log.Println("接收文件完成情况", doneSum, fileInfo.Size)
		if doneSum == fileInfo.Size {
			log.Println("消息文件接收完毕")

			// 保存入库
			kcdb.ChatMessageInfoUpdateState(m.ID, "完成")
			// 订阅回调
			feedCallback.FeedCallbackOnChatMessageState(m.FromPeerID, m.ID, "完成")

			return
		}

		// 订阅回调
		feedCallback.FeedCallbackOnChatMessageState(m.FromPeerID, m.ID, fmt.Sprintf("接收 %.0f%s", percentage*100, "%"))
	}
}

// 发送文件消息
func sendMessageFile(id string, fileInfo FileInfo, onProgress func(float64)) error {
	s, e := createStream(id, protocolIDMessageFile)
	if e != nil {
		return e
	}
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 接收对方意愿
	resultBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		return e
	}
	result := string(*resultBytes)
	if result == "拒绝" {
		return fmt.Errorf("对方拒绝")
	}

	// 写入文件信息
	fileInfoBytes, _ := json.Marshal(fileInfo)
	e = writeTextToReadWriter(rw, &fileInfoBytes)
	if e != nil {
		return e
	}

	// 写入文件数据
	f, e := os.Open(filepath.Join(fileDirectory, fmt.Sprint(fileInfo.NameWithoutExtension, ".", fileInfo.Extension)))
	if e != nil {
		return e
	}
	defer f.Close()
	var doneSum int64 //完成长度
	buf := make([]byte, 1048576)
	for {
		rn, e := f.Read(buf)
		if e != nil {
			if e == io.EOF {
				log.Println("发送会话消息文件读取本地没有更多数据")
			} else {
				log.Println("发送会话消息文件读取本地文件出错", e)
				return e
			}
		}

		var wn int
		if rn > 0 {
			wn, e = rw.Write(buf[0:rn])
			if e != nil {
				return e
			}
		}

		// 累加完成长度
		doneSum += int64(wn)
		percentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(doneSum)/float64(fileInfo.Size)), 64)

		// 计算百分比
		onProgress(percentage)

		if doneSum == fileInfo.Size {
			break
		}
	}
	e = rw.Flush()
	if e != nil {
		return e
	}

	return nil
}

// 处理文本消息
func messageTextStreamHandler(s network.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	log.Println("远程节点文本消息:", remotePeerID)
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
		log.Println("读取文本消息出错", e)
		return
	}
	text := string(*requestBytes)

	// 回复
	resultBytes := []byte("收到")
	e = writeTextToReadWriter(rw, &resultBytes)
	if e != nil {
		log.Println("读取文本消息后写入回复时出错", e)
		return
	}

	// 保存消息
	chatMessageInfo := kcdb.ChatMessageInfo{ID: time.Now().UnixNano(), FromPeerID: remotePeerID.Pretty(), ToPeerID: h.ID().Pretty(), Text: text, State: "完成", Read: false}
	e = kcdb.ChatMessageInfoInsert(&chatMessageInfo)
	if e != nil {
		log.Println("保存消息时出错", e)
		return
	}

	// 执行订阅回调
	jsonBytes, _ := json.Marshal(chatMessageInfo)
	feedCallback.FeedCallbackOnChatMessage(chatMessageInfo.FromPeerID, string(jsonBytes))
}

// 发送文本消息
func sendMessageText(id, text string) error {
	s, e := createStream(id, protocolIDMessageText)
	if e != nil {
		return e
	}
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 写入
	data := []byte(text)
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

// 发送会话消息
func sendChatMessage(m *kcdb.ChatMessageInfo) {
	log.Println("异步发送会话消息", m.ID)

	// 保存到数据库中
	e := kcdb.ChatMessageInfoInsert(m)
	if e != nil {
		log.Println("会话消息保存到数据库中出错", e)
		return
	}

	// 订阅回调
	jsonBytes, _ := json.Marshal(*m)
	feedCallback.FeedCallbackOnChatMessage(m.FromPeerID, string(jsonBytes))

	var se error
	if m.FileSize == 0 {
		se = sendMessageText(m.ToPeerID, m.Text)
	} else {
		se = sendMessageFile(m.ToPeerID, FileInfo{Size: m.FileSize,
			NameWithoutExtension: m.FileNameWithoutExtension,
			Extension:            m.FileExtension},
			func(p float64) {
				// 订阅回调
				feedCallback.FeedCallbackOnChatMessageState(m.FromPeerID, m.ID, fmt.Sprintf("发送 %.0f%s", p*100, "%"))
			})
	}

	if se != nil {
		// 保存入库
		kcdb.ChatMessageInfoUpdateState(m.ID, "失败")
		// 订阅回调
		feedCallback.FeedCallbackOnChatMessageState(m.FromPeerID, m.ID, "失败")
		return
	}

	// 保存入库
	kcdb.ChatMessageInfoUpdateState(m.ID, "完成")
	// 订阅回调
	feedCallback.FeedCallbackOnChatMessageState(m.FromPeerID, m.ID, "完成")
}

// 处理交换信息请求
func infoStreamHandler(s network.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	log.Println("远程节点交换信息:", remotePeerID)
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 获取我的信息
	option := kcoption.Get()

	// 检查拒绝名单
	if valueInArray(remotePeerID.Pretty(), kcoption.Get().BlacklistIDArray) {
		log.Println("已在拒绝名单中的远程节点:", remotePeerID)
		resultBytes := []byte("拒绝")
		writeTextToReadWriter(rw, &resultBytes)
		s.Close()
		return
	}

	// 检查没有设置我的信息的状况
	if option.Name == "" {
		log.Println("我的信息没有设置")
		resultBytes := []byte("没有信息")
		writeTextToReadWriter(rw, &resultBytes)
		s.Close()
		return
	}

	// 读取
	requestBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		log.Println("读取对方信息出错", e)
		return
	}

	// 解析
	var targetInfo kccontact.Contact
	e = json.Unmarshal(*requestBytes, &targetInfo)
	if e != nil {
		log.Println("解析对方信息出错", e)
		return
	}

	// 回复我的信息
	data, _ := json.Marshal(kccontact.Contact{ID: h.ID().Pretty(), Name: option.Name, Photo: option.Photo})
	e = writeTextToReadWriter(rw, &data)
	if e != nil {
		log.Println("回复我的信息出错", e)
	}

	// 保存对方信息
	targetInfo.ID = remotePeerID.Pretty()
	if kccontact.Has(targetInfo.ID) {
		targetInfo.NameRemark = kccontact.Get(targetInfo.ID).NameRemark // 保留名字备注
	}
	kccontact.Set(targetInfo)

	// 执行订阅回调
	jsonBytes, _ := json.Marshal(targetInfo)
	feedCallback.FeedCallbackOnContactUpdate(string(jsonBytes))
}

// 交换信息(成功时自动保存)
func exchangeInfo(id string) (*kccontact.Contact, error) {
	// 获取我的信息
	option := kcoption.Get()
	if option.Name == "" {
		return nil, fmt.Errorf("我的名字没有设置")
	}
	if valueInArray(id, kcoption.Get().BlacklistIDArray) {
		return nil, fmt.Errorf("对方正在黑名单中, 请先从黑名单中移除")
	}

	// 创建流
	s, e := createStream(id, protocolIDInfo)
	if e != nil {
		return nil, e
	}
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 发送我的信息
	data, _ := json.Marshal(kccontact.Contact{ID: h.ID().Pretty(), Name: option.Name, Photo: option.Photo})
	e = writeTextToReadWriter(rw, &data)
	if e != nil {
		return nil, e
	}

	// 接收对方信息
	resultBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		return nil, e
	}

	// 检查异常状态
	if string(*resultBytes) == "拒绝" {
		return nil, fmt.Errorf("拒绝")
	} else if string(*resultBytes) == "没有信息" {
		return nil, fmt.Errorf("没有信息")
	}

	// 解析对方信息
	var targetInfo kccontact.Contact
	e = json.Unmarshal(*resultBytes, &targetInfo)
	if e != nil {
		return nil, e
	}

	// 保存对方信息
	targetInfo.ID = id
	kccontact.Set(targetInfo)

	// 执行订阅回调
	jsonBytes, _ := json.Marshal(targetInfo)
	feedCallback.FeedCallbackOnContactUpdate(string(jsonBytes))

	return &targetInfo, nil
}

// Start 启动
// 注意: 这是阻塞方法,只有报错才会退出.
// 参考 https://github.com/libp2p/go-libp2p-examples/blob/master/libp2p-host/host.go#L20
func Start(safeDir, fileDir string, port int, callback FeedCallback) error {
	log.Printf("节点启动-安全目录:%s, 文件目录:%s, 端口:%d", safeDir, fileDir, port)

	fileDirectory = fileDir
	feedCallback = callback

	// 加载(创建)密钥
	privateKey, e := getPrivateKey(filepath.Join(safeDir, "pi.key"))
	if e != nil {
		return e
	}

	// 上下文控制libp2p节点的生命周期, 取消它可以停止节点.
	ctx, ctxCancel = context.WithCancel(context.Background())

	// 创建主机
	h, e = libp2p.New(ctx,
		// Use the keypair we generated
		libp2p.Identity(*privateKey),
		// // 没有地址
		// libp2p.ListenAddrs(),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			fmt.Sprint("/ip4/0.0.0.0/tcp/", port),          // regular tcp connections
			fmt.Sprint("/ip4/0.0.0.0/udp/", port, "/quic"), // a UDP endpoint for the QUIC transport
			fmt.Sprint("/ip6/::/udp/", port, "/quic"),      // a UDP endpoint for the QUIC transport
		),
		// support TLS connections
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		// support any other default transports (TCP)
		libp2p.DefaultTransports,
		// support QUIC - experimental
		libp2p.Transport(libp2pquic.NewTransport),
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(connmgr.NewConnManager(
			150,         // Lowwater
			300,         // HighWater,
			time.Minute, // GracePeriod
		)),
		// Attempt to open ports using uPNP for NATed hosts.
		libp2p.NATPortMap(),
		// Let this host use the DHT to find other hosts
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, e = dht.New(ctx, h)
			return idht, e
		}),
		// Let this host use relays and advertise itself on relays if
		// it finds it is behind NAT. Use libp2p.Relay(options...) to
		// enable active relays and more.
		libp2p.EnableAutoRelay(),
	)
	if e != nil {
		return fmt.Errorf("创建主机出错: %w", e)
	}

	// 创建数据库
	e = kcdb.Open(filepath.Join(safeDir, "db.sqlite"))
	if e != nil {
		return fmt.Errorf("创建数据库出错: %w", e)
	}

	// 创建选项
	kcoption.New(filepath.Join(safeDir, "option.json"))
	// 创建联系人
	kccontact.New(filepath.Join(safeDir, "contact.json"))

	// 设置流处
	h.SetStreamHandler(protocolIDInfo, infoStreamHandler)
	h.SetStreamHandler(protocolIDMessageText, messageTextStreamHandler)
	h.SetStreamHandler(protocolIDMessageFile, messageFileStreamHandler)
	h.SetStreamHandler(protocolIDRemoteControl, remoteControlStreamHandler)

	// 创建自动NAT
	_, e = autonat.New(ctx, h)
	if e != nil {
		return fmt.Errorf("创建公网穿透出错: %w", e)
	}

	// 连接引导节点, 帮助节点之间发现
	go connectBootstrap("/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ")
	go connectBootstrap("/ip4/47.74.132.162/udp/6666/quic/p2p/12D3KooWFTmSZHheeSjGvnZzBtK7UHQUQTA3XNjMwz5bEqTTRPPA")
	// 注意：android中使用dnsaddr时无法连接。
	// go connectBootstrap("/dnsaddr/bootstrap.pl.app.lilu.red/p2p/12D3KooWFTmSZHheeSjGvnZzBtK7UHQUQTA3XNjMwz5bEqTTRPPA")
	for _, v := range kcoption.Get().BootstrapArray {
		go connectBootstrap(v)
	}

	// 创建mDNS服务并注册发现列队, 帮助局域网内节点相互发现
	// 注意: mDNS依赖tcp传输!
	mdnsService, e = discovery.NewMdnsService(ctx, h, time.Second*3, mDNSServiceTag)
	if e != nil {
		return fmt.Errorf("创建内网发现出错|%w", e)
	}
	mdn = &mDNSDiscoveryNotifee{}
	mdn.PeerChan = make(chan peer.AddrInfo)
	mdnsService.RegisterNotifee(mdn)
	go func() {
		for {
			select {
			case <-stopChan:
				return
			case addrInfo := <-mdn.PeerChan:
				if valueInArray(addrInfo.ID.Pretty(), kcoption.Get().BlacklistIDArray) {
					break
				}

				connCount := len(h.Network().ConnsToPeer(addrInfo.ID))
				if connCount == 0 {
					localContext, localContextCancel := context.WithTimeout(ctx, time.Second*2)
					defer localContextCancel()
					e = h.Connect(localContext, addrInfo)
					if e != nil {
						log.Println("内网节点连接失败", e)
						clearPeerNetworkCache(addrInfo.ID)
					} else {
						log.Println("内网节点连接成功", addrInfo.ID.Pretty())

						// 防止连接被清理
						h.ConnManager().TagPeer(addrInfo.ID, connProtectTagKeep, 100)
						h.ConnManager().Protect(addrInfo.ID, connProtectTagKeep)

						// 交换信息
						_, e = exchangeInfo(addrInfo.ID.Pretty())
						if e != nil {
							log.Println("内网节点交换信息失败", e)
						}
					}
				} else if !kccontact.Has(addrInfo.ID.Pretty()) {
					//已经连接但是没有交换信息时交换
					_, e = exchangeInfo(addrInfo.ID.Pretty())
					if e != nil {
						log.Println("内网节点交换信息失败", e)
					}
				}
			}
		}
	}()

	// 开始连接状态检查
	connStateTicker = time.NewTicker(time.Second * 6)
	go connStateTickerTask()

	// 标记就绪
	ready = true

	// 保持运行
	<-ctx.Done()
	log.Println("节点停止")
	return nil
}

// Stop 停止
func Stop() {
	log.Println("停止")
	ready = false
	stopChan <- 1

	// 停止连接状态检查
	connStateTickerStopChan <- true
	connStateTicker.Stop()

	kcdb.Close()

	ctxCancel()
}

// IDValid 检查ID是否正确, 没有异常说明正确
func IDValid(peerID string) error {
	_, e := peer.Decode(peerID)
	if e != nil {
		return e
	}
	return nil
}

// GetID 获取ID(节点没有就绪时返回空字符串)
func GetID() string {
	if !ready || h == nil {
		return ""
	}

	return h.ID().Pretty()
}
