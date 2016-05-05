package server

import (
	"asicdServices"
	"errors"
	"git.apache.org/thrift.git/lib/go/thrift"
	"github.com/google/gopacket"
	_ "github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	nanomsg "github.com/op/go-nanomsg"
	"l2/lldp/packet"
	_ "net"
	"sync"
	"time"
	"utils/dbutils"
)

type LLDPClientJson struct {
	Name string `json:Name`
	Port int    `json:Port`
}

type LLDPClientBase struct {
	Address            string
	Transport          thrift.TTransport
	PtrProtocolFactory *thrift.TBinaryProtocolFactory
	IsConnected        bool
}

type LLDPAsicdClient struct {
	LLDPClientBase
	ClientHdl *asicdServices.ASICDServicesClient
}

type InPktChannel struct {
	pkt     gopacket.Packet
	ifIndex int32
}

type SendPktChannel struct {
	ifIndex int32
}

type LLDPGlobalInfo struct {
	// Port information
	PortNum       int32
	IfIndex       int32
	Name          string
	MacAddr       string // This is our Port MAC ADDR
	OperState     string
	OperStateLock *sync.RWMutex
	// Pcap Handler for Each Port
	PcapHandle *pcap.Handle
	// Pcap Handler lock to write data one routine at a time
	PcapHdlLock *sync.RWMutex
	// rx information
	RxInfo *packet.RX
	// tx information
	TxInfo *packet.TX
}

type LLDPServer struct {
	// Basic server start fields
	lldpDbHdl      *dbutils.DBUtil
	paramsDir      string
	asicdClient    LLDPAsicdClient
	asicdSubSocket *nanomsg.SubSocket

	// lldp per port global info
	lldpGblInfo           map[int32]LLDPGlobalInfo
	lldpIntfStateSlice    []int32
	lldpUpIntfStateSlice  []int32
	lldpPortNumIfIndexMap map[int32]int32
	//packet                packet.Packet

	// lldp pcap handler default config values
	lldpSnapshotLen int32
	lldpPromiscuous bool
	lldpTimeout     time.Duration

	// lldp packet rx channel
	lldpRxPktCh chan InPktChannel

	// lldp send packet channel
	lldpTxPktCh chan SendPktChannel

	// lldp exit
	lldpExit chan bool
}

const (
	// LLDP profiling
	LLDP_CPU_PROFILE_FILE = "/var/log/lldp.prof"

	// Error Message
	LLDP_USR_CONF_DB                    = "/UsrConfDb.db"
	LLDP_CLIENT_CONNECTION_NOT_REQUIRED = "Connection to Client is not required"

	// Consts Init Size/Capacity
	LLDP_INITIAL_GLOBAL_INFO_CAPACITY = 100
	LLDP_RX_PKT_CHANNEL_SIZE          = 10
	LLDP_TX_PKT_CHANNEL_SIZE          = 10

	// Port Operation State
	LLDP_PORT_STATE_DOWN = "DOWN"
	LLDP_PORT_STATE_UP   = "UP"

	LLDP_BPF_FILTER                 = "ether proto 0x88cc"
	LLDP_PROTO_DST_MAC              = "01:80:c2:00:00:0e"
	LLDP_MAX_TTL                    = 65535
	LLDP_DEFAULT_TX_INTERVAL        = 30
	LLDP_DEFAULT_TX_HOLD_MULTIPLIER = 4
	LLDP_MIN_FRAME_LENGTH           = 12 // this is 12 bytes

	// Mandatory TLV Type
	LLDP_CHASSIS_ID_TLV_TYPE uint8 = 1
	LLDP_PORT_ID_TLV_TYPE    uint8 = 2
	LLDP_TTL_TLV_TYPE        uint8 = 3
)

var (
	LLDP_INVALID_LAYERS = errors.New("received layer are not in-sufficient for decoding packet")
)
