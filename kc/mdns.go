package kc

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery"
)

type mDNSDiscoveryNotifee struct {
	PeerChan chan peer.AddrInfo
}

// implement discovery.Notifee
func (n *mDNSDiscoveryNotifee) HandlePeerFound(pa peer.AddrInfo) {
	n.PeerChan <- pa
}

var mdn *mDNSDiscoveryNotifee

var mdnsService discovery.Service
