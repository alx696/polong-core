package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/alx696/polong-core/kc"
	"github.com/alx696/polong-core/qc"
	"github.com/gorilla/websocket"
)

var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	//跨域
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
var websocketConn *websocket.Conn
var safeDirectory string
var fileDirectory string

func main() {
	safeDirectoryFlag := flag.String("safe-directory", "", "safe directory path")
	fileDirectoryFlag := flag.String("file-directory", "", "file directory path")
	p2pPortFlag := flag.Int("p2p-port", 10000, "p2p port")
	webPortFlag := flag.Int("web-port", 10001, "web port")
	flag.Parse()

	//获取用户配置文件夹
	configDir, e := os.UserConfigDir()
	if e != nil {
		log.Fatalln("获取用户配置文件夹出错", e)
	}
	if *safeDirectoryFlag == "" {
		safeDirectory = filepath.Join(configDir, "po-long")
	} else {
		safeDirectory = *safeDirectoryFlag
	}
	os.MkdirAll(safeDirectory, os.ModePerm)
	if *fileDirectoryFlag == "" {
		fileDirectory = filepath.Join(configDir, "po-long", "file")
	} else {
		fileDirectory = *fileDirectoryFlag
	}
	os.MkdirAll(fileDirectory, os.ModePerm)

	// 启动
	go func() {
		e := kc.Start(safeDirectory, fileDirectory, *p2pPortFlag, FeedCallbackImpl{})
		if e != nil {
			log.Fatalln("启动失败:", e)
		}
	}()

	// 启动http
	go startHTTP(*webPortFlag)

	// wait for a SIGINT or SIGTERM signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
	log.Println("收到信号, 关闭程序")

	//停止
	kc.Stop()
}

func startHTTP(webPort int) {
	// 获取节点ID
	http.HandleFunc("/api1/id", func(writer http.ResponseWriter, request *http.Request) {
		var id string
		for {
			id = kc.GetID()

			if id != "" {
				break
			}

			time.Sleep(time.Second)
		}

		writer.Write([]byte(id))
	})

	// 获取状态信息
	http.HandleFunc("/api1/state", func(writer http.ResponseWriter, request *http.Request) {
		result := kc.GetState()

		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(result))
	})

	// 获取我的设置
	http.HandleFunc("/api1/option", func(writer http.ResponseWriter, request *http.Request) {
		result := kc.GetOption()

		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(result))
	})

	// 设置我的信息
	http.HandleFunc("/api1/option/info", func(writer http.ResponseWriter, request *http.Request) {
		name := request.FormValue("name")
		photo := request.FormValue("photo")

		kc.SetInfo(name, photo)
	})

	// 设置引导地址
	http.HandleFunc("/api1/option/bootstrap", func(writer http.ResponseWriter, request *http.Request) {
		arrayText := request.FormValue("arrayText")

		e := kc.SetBootstrapArray(arrayText)
		if e != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(e.Error()))
			return
		}
	})

	// 设置黑名单
	http.HandleFunc("/api1/option/blacklist", func(writer http.ResponseWriter, request *http.Request) {
		arrayText := request.FormValue("arrayText")

		kc.SetBlacklistIDArray(arrayText)
	})

	// 订阅推送
	http.HandleFunc("/api1/feed", func(writer http.ResponseWriter, request *http.Request) {
		conn, e := websocketUpgrader.Upgrade(writer, request, nil)
		if e != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(e.Error()))
			return
		}

		//读取请求
		messageType, _, e := conn.ReadMessage()
		if e != nil || messageType != websocket.TextMessage {
			log.Println("请求非法: ", e)
			_ = conn.Close()
			return
		}

		//缓存连接
		websocketConn = conn
	})

	// 联系人
	http.HandleFunc("/api1/contact", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "GET" {
			result := kc.GetContact()

			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(result))
		} else if request.Method == "DELETE" {
			id := request.URL.Query().Get("id")

			if id == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			kc.DelContact(id)
		} else if request.Method == "POST" {
			id := request.FormValue("id")

			if id == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			result, e := kc.NewContact(id)
			if e != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				writer.Write([]byte(e.Error()))
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(result))
		}
	})

	// 联系人设置备注
	http.HandleFunc("/api1/contact/name/remark", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "POST" {
			id := request.FormValue("id")
			name := request.FormValue("name")

			if id == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			kc.SetContactNameRemark(id, name)
		}
	})

	// 获取连接信息
	http.HandleFunc("/api1/contact/connect", func(writer http.ResponseWriter, request *http.Request) {
		result := kc.GetConnectedPeerIDs()

		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(result))
	})

	// 文件(供无法直接访问文件系统的客户端使用)
	http.HandleFunc("/api1/file", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "POST" {
			nameWithoutExtension := request.FormValue("nameWithoutExtension")
			extension := request.FormValue("extension")

			// 准备文件路径
			filePath := filepath.Join(fileDirectory, fmt.Sprintf("%s.%s", nameWithoutExtension, extension))
			_, e := os.Stat(filePath)
			if e == nil {
				nameWithoutExtension = fmt.Sprintf("%s[%d]", nameWithoutExtension, time.Now().Nanosecond())
			}
			filePath = filepath.Join(fileDirectory, fmt.Sprintf("%s.%s", nameWithoutExtension, extension))

			multipartFile, _, e := request.FormFile("file")
			if e != nil {
				log.Println("读取文件错误: ", e)
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(e.Error()))
				return
			}
			defer multipartFile.Close()
			buffer := bytes.NewBuffer(nil)
			if _, e := io.Copy(buffer, multipartFile); e != nil {
				log.Println("读取文件错误: ", e)
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(e.Error()))
				return
			}
			e = ioutil.WriteFile(filePath, buffer.Bytes(), 0666)
			if e != nil {
				log.Println("保存文件错误: ", e)
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(e.Error()))
				return
			}
		} else if request.Method == "GET" {
			fileName := request.URL.Query().Get("fileName")
			fileBytes, e := ioutil.ReadFile(filepath.Join(fileDirectory, fileName))
			if e != nil {
				log.Println("读取文件错误:", e)
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = writer.Write(fileBytes)
		}
	})

	// 会话消息
	http.HandleFunc("/api1/chat/message", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "POST" {
			peerID := request.FormValue("peerID")
			text := request.FormValue("text")
			nameWithoutExtension := request.FormValue("nameWithoutExtension")
			extension := request.FormValue("extension")
			size, _ := strconv.ParseInt(request.FormValue("size"), 10, 64)

			if peerID == "" || (text == "" && size == 0) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			if text != "" {
				kc.SendChatMessageText(peerID, text)
			} else if size != 0 {

				kc.SendChatMessageFile(peerID, nameWithoutExtension, extension, size)
			}
		} else if request.Method == "GET" {
			peerID := request.URL.Query().Get("peerID")

			if peerID == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			result, _ := kc.FindChatMessage(peerID)
			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(result))
		} else if request.Method == "DELETE" {
			peerID := request.URL.Query().Get("peerID")

			if peerID == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			kc.DeleteChatMessageByPeerID(peerID)
		}
	})

	// 会话消息已读状态
	http.HandleFunc("/api1/chat/message/read", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "POST" {
			peerID := request.FormValue("peerID")

			if peerID == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			kc.SetChatMessageReadByPeerID(peerID)
		} else if request.Method == "GET" {
			result, _ := kc.GetChatMessageUnReadCount()
			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(result))
		}
	})

	// 二维码
	http.HandleFunc("/api1/qrcode", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "GET" {
			text := request.URL.Query().Get("text")

			if text == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			path := filepath.Join(fileDirectory, "qrcode.jpg")
			qc.Encode(path, text)
		}
	})

	// 获取目录
	runDir, e := filepath.Abs(filepath.Dir(os.Args[0]))
	if e != nil {
		log.Fatalln(e)
	}

	// 开启页面
	webDir := filepath.Join(runDir, "web")
	_, e = os.Stat(webDir)
	if e == nil {
		log.Println("开启页面", webDir)
		http.Handle("/", http.FileServer(http.Dir(webDir)))
	}

	// 监听http
	log.Fatalln(http.ListenAndServe(fmt.Sprint(":", webPort), nil))
}

// 实现FeedCallback接口
type FeedCallbackImpl struct {
}

type PushInfo struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	ID        string `json:"id"`
	IsConnect bool   `json:"isConnect"`
	MessageID int64  `json:"messageID"`
}

func (impl FeedCallbackImpl) FeedCallbackOnContactUpdate(text string) {
	if websocketConn == nil {
		return
	}

	push := PushInfo{Type: "ContactUpdate", Text: text}
	jsonBytes, _ := json.Marshal(push)

	e := websocketConn.WriteMessage(websocket.TextMessage, jsonBytes)
	if e != nil {
		log.Println("WebSocket出错", e)
	}
}

func (impl FeedCallbackImpl) FeedCallbackOnContactDelete(id string) {
	if websocketConn == nil {
		return
	}

	push := PushInfo{Type: "ContactDelete", ID: id}
	jsonBytes, _ := json.Marshal(push)

	e := websocketConn.WriteMessage(websocket.TextMessage, jsonBytes)
	if e != nil {
		log.Println("WebSocket出错", e)
	}
}

func (impl FeedCallbackImpl) FeedCallbackOnPeerConnectState(id string, isConnect bool) {
	if websocketConn == nil {
		return
	}

	push := PushInfo{Type: "PeerConnectState", ID: id, IsConnect: isConnect}
	jsonBytes, _ := json.Marshal(push)

	e := websocketConn.WriteMessage(websocket.TextMessage, jsonBytes)
	if e != nil {
		log.Println("WebSocket出错", e)
	}
}

func (impl FeedCallbackImpl) FeedCallbackOnChatMessage(peerID string, chatMessage string) {
	if websocketConn == nil {
		return
	}

	push := PushInfo{Type: "ChatMessage", Text: chatMessage, ID: peerID}
	jsonBytes, _ := json.Marshal(push)

	e := websocketConn.WriteMessage(websocket.TextMessage, jsonBytes)
	if e != nil {
		log.Println("WebSocket出错", e)
	}
}

func (impl FeedCallbackImpl) FeedCallbackOnChatMessageState(peerID string, messageID int64, state string) {
	if websocketConn == nil {
		return
	}

	push := PushInfo{Type: "ChatMessageState", ID: peerID, MessageID: messageID, Text: state}
	jsonBytes, _ := json.Marshal(push)

	e := websocketConn.WriteMessage(websocket.TextMessage, jsonBytes)
	if e != nil {
		log.Println("WebSocket出错", e)
	}
}

func (impl FeedCallbackImpl) FeedCallbackOnRemoteControlReceiveVideoInfo(json string) {
	log.Println("收到远程控制视频信息", json)
}

func (impl FeedCallbackImpl) FeedCallbackOnRemoteControlReceiveVideoData(data []byte) {
	log.Println("收到远程控制视频数据", len(data))
}

func (impl FeedCallbackImpl) FeedCallbackOnRemoteControlRequest(peerID string) {
	log.Println("收到远程控制请求", peerID)
}
