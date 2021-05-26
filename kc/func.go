package kc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/multiformats/go-multiaddr"
)

// 值是否在数组中
func valueInArray(value string, array []string) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}

	return false
}

// 获取密钥(没有时自动生成)
func getPrivateKey(privateKeyPath string) (*crypto.PrivKey, error) {
	var privateKey crypto.PrivKey
	var privateKeyBytes []byte
	_, e = os.Stat(privateKeyPath)
	if os.IsNotExist(e) {
		privateKey, _, e = crypto.GenerateKeyPair(
			crypto.Ed25519, // Select your key type. Ed25519 are nice short
			-1,             // Select key length when possible (i.e. RSA).
		)
		if e != nil {
			return nil, fmt.Errorf("生成密钥出错|%w", e)
		}
		privateKeyBytes, e = crypto.MarshalPrivateKey(privateKey)
		if e != nil {
			return nil, fmt.Errorf("编码密钥出错|%w", e)
		}
		e = ioutil.WriteFile(privateKeyPath, privateKeyBytes, os.ModePerm)
		if e != nil {
			return nil, fmt.Errorf("保存密钥出错|%w", e)
		}
	} else {
		privateKeyBytes, e = ioutil.ReadFile(privateKeyPath)
		if e != nil {
			return nil, fmt.Errorf("读取密钥出错|%w", e)
		}
		privateKey, e = crypto.UnmarshalPrivateKey(privateKeyBytes)
		if e != nil {
			return nil, fmt.Errorf("解码密钥出错|%w", e)
		}
	}
	return &privateKey, nil
}

// 清除节点网络缓存
// 防止拨号器使用无法连接地址快速重拨导致一直连不上.
// https://github.com/prysmaticlabs/prysm/issues/2674#issuecomment-529229685
func clearPeerNetworkCache(id peer.ID) {
	h.Peerstore().ClearAddrs(id)
	h.Network().(*swarm.Swarm).Backoff().Clear(id)
}

// 多址字符转节点地址
func multiaddrToAddrInfo(multiaddrText string) (*peer.AddrInfo, error) {
	multiAddr, e := multiaddr.NewMultiaddr(multiaddrText)
	if e != nil {
		return nil, fmt.Errorf("多址字符错误: %w", e)
	}

	addrInfo, e := peer.AddrInfoFromP2pAddr(multiAddr)
	if e != nil {
		return nil, fmt.Errorf("多址字符转节点地址出错: %w", e)
	}

	// 在节点地址中添加中继多址
	relayMultiAddr, e := multiaddr.NewMultiaddr(fmt.Sprint("/p2p-circuit/ipfs/", addrInfo.ID.Pretty()))
	if e != nil {
		return nil, fmt.Errorf("创建中继多址出错: %w", e)
	}
	addrInfo.Addrs = append(addrInfo.Addrs, relayMultiAddr)

	return addrInfo, nil
}

// 连接引导(帮助节点之间进行发现)
func connectBootstrap(multiaddrText string) error {
	log.Println("连接引导地址", multiaddrText)
	addrInfo, e := multiaddrToAddrInfo(multiaddrText)
	if e != nil {
		log.Println("连接引导失败", e)
		return e
	}

	lc, lcCancel := context.WithTimeout(ctx, time.Second*16)
	defer lcCancel()
	e = h.Connect(lc, *addrInfo)
	if e != nil {
		log.Println("连接引导失败", e)
		return fmt.Errorf("连接引导失败: %w", e)
	}
	log.Println("连接引导成功", multiaddrText)

	return nil
}

// 从DHT中查找节点地址信息
func findDHTAddrInfo(id peer.ID) (*peer.AddrInfo, error) {
	localContext, localContextCancel := context.WithTimeout(ctx, time.Second*2)
	defer localContextCancel()
	addrInfo, e := idht.FindPeer(localContext, id)
	if e != nil {
		return nil, e
	}

	// 地址信息中添加中继地址, 支持没有公网IP的用户
	//参考 https://github.com/libp2p/go-libp2p-examples/blob/master/relay/main.go
	relayMultiAddr, e := multiaddr.NewMultiaddr(fmt.Sprint("/p2p-circuit/ipfs/", id.Pretty()))
	if e != nil {
		return nil, e
	}
	addrInfo.Addrs = append(addrInfo.Addrs, relayMultiAddr)

	return &addrInfo, nil
}

// 连接DHT节点(用于保持连接状态)
func connectDHTPeer(id peer.ID) error {
	addrInfo, e := findDHTAddrInfo(id)
	if e != nil {
		clearPeerNetworkCache(id)
		return e
	}

	localContext, localContextCancel := context.WithTimeout(ctx, time.Second*1)
	defer localContextCancel()
	e = h.Connect(localContext, *addrInfo)
	if e != nil {
		clearPeerNetworkCache(id)
		return e
	}

	// 标记并保护指定节点, 防止连接被清理
	h.ConnManager().TagPeer(id, connProtectTagKeep, 100)
	h.ConnManager().Protect(id, connProtectTagKeep)

	return nil
}

// 创建节点的流
// 注意: defer s.Close()
func createStream(id string, protocolID protocol.ID) (network.Stream, error) {
	peerID, _ := peer.Decode(id)
	lc, lcCancel := context.WithTimeout(ctx, time.Second*3)
	defer lcCancel()
	return h.NewStream(lc, peerID, protocolID)
}

// 从读写器中获取文本
func readTextFromReadWriter(rw *bufio.ReadWriter) (*[]byte, error) {
	//读取
	txt, e := rw.ReadString('\n')
	if e != nil {
		return nil, e
	}
	//移除delim
	txt = strings.TrimSuffix(txt, "\n")

	if txt == "" {
		var empty []byte
		return &empty, nil
	}

	// //读取
	// encodeData, e := rw.ReadBytes('\n')
	// if e != nil {
	// 	return nil, e
	// }
	// //移除delim
	// encodeData = encodeData[0 : len(encodeData)-1]

	//解码
	data, e := base64.StdEncoding.DecodeString(txt)
	if e != nil {
		return nil, e
	}

	return &data, nil
}

// 往读写器中写入文本
func writeTextToReadWriter(rw *bufio.ReadWriter, data *[]byte) error {
	//编码
	encodeData := []byte(base64.StdEncoding.EncodeToString(*data))

	//添加delim
	encodeData = append(encodeData, '\n')

	//写入
	_, e = rw.Write(encodeData)
	if e != nil {
		return e
	}
	e = rw.Flush()
	if e != nil {
		return e
	}
	return nil
}

// 从读写器中读取数据
func readDataFromReadWriter(rw *bufio.ReadWriter) (*[]byte, error) {
	var data []byte
	scanner := bufio.NewScanner(rw.Reader)
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
	}

	return &data, nil
}

// 往读写器中写入数据
func writeDataToReadWriter(rw *bufio.ReadWriter, data *[]byte) error {
	//添加delim
	finalData := append(*data, '\n')

	//写入
	_, e = rw.Write(finalData)
	if e != nil {
		return e
	}
	e = rw.Flush()
	if e != nil {
		return e
	}
	return nil
}

// 构建视频元信息
func toVideoMetadata(presentationTimeUs, size int64) []byte {
	timeRealBytes := []byte(strconv.FormatInt(presentationTimeUs, 10))
	timeBytes := make([]byte, 64)
	for i := 0; i < len(timeBytes); i++ {
		if i < len(timeRealBytes) {
			timeBytes[i] = timeRealBytes[i]
		}
	}

	sizeRealBytes := []byte(strconv.FormatInt(size, 10))
	sizeBytes := make([]byte, 64)
	for i := 0; i < len(sizeBytes); i++ {
		if i < len(sizeRealBytes) {
			sizeBytes[i] = sizeRealBytes[i]
		}
	}

	var packageData []byte
	packageData = append(packageData, timeBytes...)
	packageData = append(packageData, sizeBytes...)

	return packageData
}

// 解析视频元信息
func fromVideoMetadata(metadataData []byte) (int64, int64, error) {
	presentationTimeUsText := string(bytes.TrimRight(metadataData[0:64], string(rune(0))))
	sizeText := string(bytes.TrimRight(metadataData[64:128], string(rune(0))))

	presentationTimeUs, e := strconv.ParseInt(presentationTimeUsText, 10, 64)
	if e != nil {
		return 0, 0, e
	}

	size, e := strconv.ParseInt(sizeText, 10, 64)
	if e != nil {
		return 0, 0, e
	}

	return presentationTimeUs, size, nil
}
