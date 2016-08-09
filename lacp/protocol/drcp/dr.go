//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// _______  __       __________   ___      _______.____    __    ____  __  .___________.  ______  __    __
// |   ____||  |     |   ____\  \ /  /     /       |\   \  /  \  /   / |  | |           | /      ||  |  |  |
// |  |__   |  |     |  |__   \  V  /     |   (----` \   \/    \/   /  |  | `---|  |----`|  ,----'|  |__|  |
// |   __|  |  |     |   __|   >   <       \   \      \            /   |  |     |  |     |  |     |   __   |
// |  |     |  `----.|  |____ /  .  \  .----)   |      \    /\    /    |  |     |  |     |  `----.|  |  |  |
// |__|     |_______||_______/__/ \__\ |_______/        \__/  \__/     |__|     |__|      \______||__|  |__|
//
// dr.go
package drcp

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	//"fmt"
	"github.com/google/gopacket/layers"
	"l2/lacp/protocol/lacp"
	"l2/lacp/protocol/utils"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
)

var DistributedRelayDB map[string]*DistributedRelay
var DistributedRelayDBList []*DistributedRelay

// holds the current conversation map values
var ConversationIdMap [MAX_CONVERSATION_IDS]ConvIdTypeValue

type ConvIdTypeValue struct {
	valid      bool
	idtype     GatewayAlgorithm
	isid       uint32
	cvlan      uint16
	svlan      uint16
	bvid       uint16
	psuedowire uint32
	portList   []int32
}

// 802.1ax-2014 7.4.1.1
type DistributedRelay struct {
	DistributedRelayFunction
	DrniId          uint32
	DrniDescription string
	DrniName        string

	// Also defined in 9.4.7
	DrniAggregatorId        [6]uint8
	DrniAggregatorPriority  uint16
	DrniPortalAddr          net.HardwareAddr
	DrniPortalPriority      uint16
	DrniThreeSystemPortal   bool
	DrniPortConversation    [MAX_CONVERSATION_IDS][4]uint8
	DrniGatewayConversation [MAX_CONVERSATION_IDS][4]uint8
	// End also defined in 9.4.7

	// save the origional values from the aggregator
	PrevAggregatorId       [6]uint8
	PrevAggregatorPriority uint16

	DrniPortalSystemNumber  uint8                 // 1-3
	DrniIntraPortalLinkList [MAX_IPP_LINKS]uint32 // ifindex
	DrniAggregator          int32
	DrniConvAdminGateway    [MAX_CONVERSATION_IDS][MAX_PORTAL_SYSTEM_IDS]uint8
	// conversation id -> gateway
	DrniNeighborAdminConvGatewayListDigest Md5Digest
	DrniNeighborAdminConvPortListDigest    Md5Digest
	DrniGatewayAlgorithm                   GatewayAlgorithm
	DrniNeighborAdminGatewayAlgorithm      GatewayAlgorithm
	DrniNeighborAdminPortAlgorithm         GatewayAlgorithm
	DrniNeighborAdminDRCPState             uint8
	DrniEncapMethod                        EncapMethod
	DrniIPLEncapMap                        map[uint32]uint32
	DrniNetEncapMap                        map[uint32]uint32
	DrniPSI                                bool
	DrniPortConversationControl            bool
	DrniPortalPortProtocolIDA              net.HardwareAddr

	GatewayVectorDatabase []NeighborGatewayVector

	// 9.4.10
	PortConversationUpdate    bool
	IppAllPortUpdate          bool
	GatewayConversationUpdate bool
	IppAllGatewayUpdate       bool

	// channel used to wait on response from distributed event send
	drEvtResponseChan chan string

	a *lacp.LaAggregator

	// sync creation and deletion
	wg sync.WaitGroup

	// state machines
	PsMachineFsm *PsMachine
	GMachineFsm  *GMachine
	AMachineFsm  *AMachine

	Ipplinks []*DRCPIpp
}

// 802.1ax-2014 Section 9.4.8 Per-DR Function variables
type DistributedRelayFunction struct {
	ChangeDRFPorts                                bool
	ChangePortal                                  bool
	DrniCommonMethods                             bool
	DrniConversationGatewayList                   [MAX_CONVERSATION_IDS]uint32
	DrniPortalSystemState                         [4]NeighborStateInfo
	DRFHomeAdminAggregatorKey                     uint16
	DRFHomeConversationGatewayListDigest          Md5Digest
	DRFHomeConversationPortListDigest             Md5Digest
	DRFHomeGatewayAlgorithm                       [4]uint8
	DRFHomeGatewayConversationMask                [MAX_CONVERSATION_IDS]bool
	DRFHomeGatewaySequence                        uint16
	DRFHomePortAlgorithm                          [4]uint8
	DRFHomeOperAggregatorKey                      uint16
	DRFHomeOperPartnerAggregatorKey               uint16
	DRFHomeState                                  NeighborStateInfo
	DRFNeighborAdminConversationGatewayListDigest Md5Digest
	DRFNeighborAdminConversationPortListDigest    Md5Digest
	DRFNeighborAdminDRCPState                     layers.DRCPState
	DRFNeighborAdminGatewayAlgorithm              [4]uint8
	DRFNeighborAdminPortAlgorithm                 [4]uint8
	// range 1..3
	DRFPortalSystemNumber uint8
	DRFHomeOperDRCPState  layers.DRCPState
	PSI                   bool

	// 9.3.3.2
	DrniPortalSystemGatewayConversation [MAX_CONVERSATION_IDS]bool
	DrniPortalSystemPortConversation    [MAX_CONVERSATION_IDS]bool
}

// DrFindByPortalAddr each portal address is unique within the system
func DrFindByPortalAddr(portaladdr string, dr **DistributedRelay) bool {
	for _, d := range DistributedRelayDBList {
		if d.DrniPortalAddr.String() == portaladdr {
			*dr = d
			return true
		}
	}
	return false
}

// GetAllCVIDConversations: Fill in the mapping of vlan -> conversation id which is 1:1
func (dr *DistributedRelay) GetAllCVIDConversations() {
	curMark := 0
	count := 100
	more := true
	for more {
		for _, client := range utils.GetAsicDPluginList() {

			bulkVlanInfo, _ := client.GetBulkVlan(curMark, count)
			if bulkVlanInfo != nil {
				objCnt := int(bulkVlanInfo.Count)
				more = bool(bulkVlanInfo.More)
				curMark = int(bulkVlanInfo.EndIdx)
				for i := 0; i < objCnt; i++ {
					vlan := bulkVlanInfo.VlanList[i].VlanId

					ent := ConversationIdMap[uint16(vlan)]
					ent.valid = true
					ent.idtype = GATEWAY_ALGORITHM_CVID
					ent.cvlan = uint16(vlan)
					// TODO should look at tagged list as well?
					if ent.portList == nil {
						ent.portList = make([]int32, 0)
					}
					untaggedIfIndexList := bulkVlanInfo.VlanList[i].UntagIfIndexList
					for _, ifindex := range untaggedIfIndexList {
						ent.portList = append(ent.portList, ifindex)
					}
					ConversationIdMap[uint16(vlan)] = ent
				}
			}
		}
	}
}

// setTimeSharingGatwewayDigest, when the port and gateway algorithm
// is set to time sharing then it should be noted that the gateway
// and port algorithm digest
// currently we only support Vlan based
// to start each
// algorithm is as follows:
// Conversations are not bound to a lag link but rather a portal system,
// thus all down traffic will either go to the local aggregator ports
// or IPL if the destination is a remote portal network port (which is not
// an aggregator port).  All up traffic is only destined to another
// aggregator or other network links either in hte local system or accross
// the IPL to the neighbor system.
// If all local aggregator ports are down then the neighbor system must
// forward frames out the aggregator as well as any network links to
// which the frame is destined for
func (dr *DistributedRelay) setTimeSharingPortAndGatwewayDigest() {
	// algorithm assumes 2P system only
	if dr.DrniGatewayAlgorithm == GATEWAY_ALGORITHM_CVID {
		dr.GetAllCVIDConversations()
		if !dr.DrniThreeSystemPortal {
			dr.setAdminConvGatewayAndNeighborGatewayListDigest()
		}
	}
}

// setAdminConvGatewayAndNeighborGatewayListDigest will set the predetermined
// algorithm as the gateway.  Every even vlan will have its gateway in system
// 2 and every odd vlan will have its gateway in system 1
func (dr *DistributedRelay) setAdminConvGatewayAndNeighborGatewayListDigest() {
	ghash := md5.New()
	for cid, conv := range ConversationIdMap {
		if conv.valid {
			if math.Mod(float64(conv.cvlan), 2) == 0 {
				dr.DrniConvAdminGateway[cid] = [3]uint8{2, 1}
			} else {
				dr.DrniConvAdminGateway[cid] = [3]uint8{1, 2}
			}

			buf := new(bytes.Buffer)
			// network byte order
			binary.Write(buf, binary.BigEndian, []uint8{dr.DrniConvAdminGateway[cid][0], dr.DrniConvAdminGateway[cid][1], uint8(cid >> 8 & 0xff), uint8(cid & 0xff)})
			ghash.Write(buf.Bytes())
		} else {
			buf := new(bytes.Buffer)
			// network byte order
			binary.Write(buf, binary.BigEndian, []uint8{uint8(cid >> 8 & 0xff), uint8(cid & 0xff)})
			ghash.Write(buf.Bytes())
		}
	}
	for i, val := range ghash.Sum(nil) {
		dr.DrniNeighborAdminConvGatewayListDigest[i] = val
		dr.DRFNeighborAdminConversationGatewayListDigest[i] = val
		dr.DRFHomeConversationGatewayListDigest[i] = val
	}
}

// NewDistributedRelay create a new instance of Distributed Relay and
// the associated objects for the IPP ports
func NewDistributedRelay(cfg *DistrubtedRelayConfig) *DistributedRelay {
	dr := &DistributedRelay{
		DrniId:                      uint32(cfg.DrniPortalSystemNumber),
		DrniName:                    cfg.DrniName,
		DrniPortalPriority:          cfg.DrniPortalPriority,
		DrniThreeSystemPortal:       cfg.DrniThreePortalSystem,
		DrniPortalSystemNumber:      cfg.DrniPortalSystemNumber,
		DrniIntraPortalLinkList:     cfg.DrniIntraPortalLinkList,
		DrniAggregator:              int32(cfg.DrniAggregator),
		DrniConvAdminGateway:        cfg.DrniConvAdminGateway,
		DrniPortConversationControl: cfg.DrniPortConversationControl,
		drEvtResponseChan:           make(chan string),
		DrniIPLEncapMap:             make(map[uint32]uint32),
		DrniNetEncapMap:             make(map[uint32]uint32),
	}
	dr.DrniPortalAddr, _ = net.ParseMAC(cfg.DrniPortalAddress)

	// string format in bits "00000000"
	for i, j := 0, uint32(7); i < 8; i, j = i+1, j-1 {
		val, _ := strconv.Atoi(cfg.DrniNeighborAdminDRCPState[i : i+1])
		dr.DrniNeighborAdminDRCPState |= uint8(val << j)
		dr.DRFNeighborAdminDRCPState |= layers.DRCPState(val << j)
	}

	// This should be nil
	for i := 0; i < 16; i++ {
		dr.DrniNeighborAdminConvPortListDigest[i] = cfg.DrniNeighborAdminConvPortListDigest[i]
	}
	// format "00:00:00:00"
	encapmethod := strings.Split(cfg.DrniEncapMethod, ":")
	gatewayalgorithm := strings.Split(cfg.DrniGatewayAlgorithm, ":")
	neighborgatewayalgorithm := strings.Split(cfg.DrniNeighborAdminGatewayAlgorithm, ":")
	//neighborportalgorithm := strings.Split(cfg.DrniNeighborAdminPortAlgorithm, ":")
	var val1, val2, val3, val4 int64
	val1, _ = strconv.ParseInt(encapmethod[0], 16, 16)
	val2, _ = strconv.ParseInt(encapmethod[1], 16, 16)
	val3, _ = strconv.ParseInt(encapmethod[2], 16, 16)
	val4, _ = strconv.ParseInt(encapmethod[3], 16, 16)
	dr.DrniEncapMethod = [4]uint8{uint8(val1), uint8(val2), uint8(val3), uint8(val4)}
	val1, _ = strconv.ParseInt(gatewayalgorithm[0], 16, 16)
	val2, _ = strconv.ParseInt(gatewayalgorithm[1], 16, 16)
	val3, _ = strconv.ParseInt(gatewayalgorithm[2], 16, 16)
	val4, _ = strconv.ParseInt(gatewayalgorithm[3], 16, 16)
	dr.DrniGatewayAlgorithm = [4]uint8{uint8(val1), uint8(val2), uint8(val3), uint8(val4)}
	val1, _ = strconv.ParseInt(neighborgatewayalgorithm[0], 16, 16)
	val2, _ = strconv.ParseInt(neighborgatewayalgorithm[1], 16, 16)
	val3, _ = strconv.ParseInt(neighborgatewayalgorithm[2], 16, 16)
	val4, _ = strconv.ParseInt(neighborgatewayalgorithm[3], 16, 16)
	dr.DrniNeighborAdminGatewayAlgorithm = [4]uint8{uint8(val1), uint8(val2), uint8(val3), uint8(val4)}
	dr.DRFNeighborAdminGatewayAlgorithm = [4]uint8{uint8(val1), uint8(val2), uint8(val3), uint8(val4)}

	for i, data := range cfg.DrniIPLEncapMap {
		dr.DrniIPLEncapMap[uint32(i)] = data
	}
	for i, data := range cfg.DrniNetEncapMap {
		dr.DrniNetEncapMap[uint32(i)] = data
	}

	netMac, _ := net.ParseMAC(cfg.DrniIntraPortalPortProtocolDA)
	dr.DrniPortalPortProtocolIDA = netMac

	// set gateway info and digest
	dr.setTimeSharingPortAndGatwewayDigest()

	// add to the global db's
	DistributedRelayDB[dr.DrniName] = dr
	DistributedRelayDBList = append(DistributedRelayDBList, dr)

	for _, ippid := range dr.DrniIntraPortalLinkList {
		if ippid > 0 {
			ipp := NewDRCPIpp(ippid, dr)
			// disabled until an aggregator has been attached
			ipp.DRCPEnabled = false
			dr.Ipplinks = append(dr.Ipplinks, ipp)
		}
	}
	return dr
}

// DeleteDistriutedRelay will delete the distributed relay along with
// the associated IPP links and de-associate from the Aggregator
func (dr *DistributedRelay) DeleteDistributedRelay() {
	dr.Stop()

	for _, ipp := range dr.Ipplinks {
		ipp.DeleteDRCPIpp()
	}

	// cleanup the tables hosting the dr data
	// cleanup the tables
	if _, ok := DistributedRelayDB[dr.DrniName]; ok {
		delete(DistributedRelayDB, dr.DrniName)
		for i, deldr := range DistributedRelayDBList {
			if deldr == dr {
				DistributedRelayDBList = append(DistributedRelayDBList[:i], DistributedRelayDBList[i+1:]...)
			}
		}
	}
}

// BEGIN will start/build all the Distributed Relay State Machines and
// send the begin event
func (dr *DistributedRelay) BEGIN(restart bool) {

	mEvtChan := make([]chan utils.MachineEvent, 0)
	evt := make([]utils.MachineEvent, 0)

	// there is a case in which we have only called
	// restart and called main functions outside
	// of this scope (TEST for example)
	//prevBegin := p.begin

	// System in being initalized
	//p.begin = true

	if !restart {
		// Portal System Machine
		dr.DrcpPsMachineMain()
		// Gateway Machine
		dr.DrcpGMachineMain()
		// Aggregator Machine
		dr.DrcpAMachineMain()
	}

	// wait group used when stopping all the
	// State mahines associated with this port.
	// want to ensure that all routines are stopped
	// before proceeding with cleanup thus why not
	// create the wg as part of a BEGIN process
	// 1) Portal System Machine
	// 2) Gateway Machine
	// 3) Aggregator Machine
	// Psm
	if dr.PsMachineFsm != nil {
		mEvtChan = append(mEvtChan, dr.PsMachineFsm.PsmEvents)
		evt = append(evt, utils.MachineEvent{
			E:   PsmEventBegin,
			Src: DRCPConfigModuleStr})
	}

	// Gm
	if dr.GMachineFsm != nil {
		mEvtChan = append(mEvtChan, dr.GMachineFsm.GmEvents)
		evt = append(evt, utils.MachineEvent{
			E:   GmEventBegin,
			Src: DRCPConfigModuleStr})
	}
	// Am
	if dr.AMachineFsm != nil {
		mEvtChan = append(mEvtChan, dr.AMachineFsm.AmEvents)
		evt = append(evt, utils.MachineEvent{
			E:   AmEventBegin,
			Src: DRCPConfigModuleStr})
	}
	// call the begin event for each
	// distribute the port disable event to various machines
	dr.DistributeMachineEvents(mEvtChan, evt, true)
}

func (dr *DistributedRelay) waitgroupadd(m string) {
	//fmt.Println("Calling wait group add", m)
	dr.wg.Add(1)
}

func (dr *DistributedRelay) waitgroupstop(m string) {
	//fmt.Println("Calling wait group stop", m)
	dr.wg.Done()
}

func (dr *DistributedRelay) Stop() {
	// Psm
	if dr.PsMachineFsm != nil {
		dr.PsMachineFsm.Stop()
		dr.PsMachineFsm = nil
	}
	// Gm
	if dr.GMachineFsm != nil {
		dr.GMachineFsm.Stop()
		dr.GMachineFsm = nil
	}
	// Am
	if dr.AMachineFsm != nil {
		dr.AMachineFsm.Stop()
		dr.AMachineFsm = nil
	}
	dr.wg.Wait()

	close(dr.drEvtResponseChan)
}

// DistributeMachineEvents will distribute the events in parrallel
// to each machine
func (dr *DistributedRelay) DistributeMachineEvents(mec []chan utils.MachineEvent, e []utils.MachineEvent, waitForResponse bool) {

	length := len(mec)
	if len(mec) != len(e) {
		dr.LaDrLog("LADR: Distributing of events failed")
		return
	}

	// send all begin events to each machine in parrallel
	for j := 0; j < length; j++ {
		go func(d *DistributedRelay, w bool, idx int, machineEventChannel []chan utils.MachineEvent, event []utils.MachineEvent) {
			if w {
				event[idx].ResponseChan = d.drEvtResponseChan
			}
			event[idx].Src = DRCPConfigModuleStr
			machineEventChannel[idx] <- event[idx]
		}(dr, waitForResponse, j, mec, e)
	}

	if waitForResponse {
		i := 0
		// lets wait for all the machines to respond
		for {
			select {
			case mStr := <-dr.drEvtResponseChan:
				i++
				dr.LaDrLog(strings.Join([]string{"LADR:", mStr, "response received"}, " "))
				//fmt.Println("LAPORT: Waiting for response Delayed", length, "curr", i, time.Now())
				if i >= length {
					// 10/24/15 fixed hack by sending response after Machine.ProcessEvent
					// HACK, found that port is pre-empting the State machine callback return
					// lets delay for a short period to allow for event to be received
					// and other routines to process their events
					/*
						if p.logEna {
							time.Sleep(time.Millisecond * 3)
						} else {
							time.Sleep(time.Millisecond * 1)
						}
					*/
					return
				}
			}
		}
	}
}

// getNeighborVectorGatwaySequenceIndex get the index for the entry whos
// sequence number is equal.
func (dr *DistributedRelay) getNeighborVectorGatwaySequenceIndex(sequence uint32, vector []bool) int32 {
	if len(dr.GatewayVectorDatabase) > 0 {
		for i, seqVector := range dr.GatewayVectorDatabase {
			if seqVector.Sequence == sequence {
				return int32(i)
			}
		}
	}
	return -1
}

// updateNeighborVector will update the vector, indexed by the received
// Home_Gateway_Sequence in increasing sequence number order
func (dr *DistributedRelay) updateNeighborVector(sequence uint32, vector []bool) {

	if len(dr.GatewayVectorDatabase) > 0 {
		for i, seqVector := range dr.GatewayVectorDatabase {
			if seqVector.Sequence == sequence {
				// overwrite the sequence
				dr.GatewayVectorDatabase[i] = NeighborGatewayVector{
					Sequence: sequence,
				}
				for j, val := range vector {
					dr.GatewayVectorDatabase[i].Vector[j] = val
				}
			} else if seqVector.Sequence > sequence {
				// insert sequence/vecotor before between i and i -1
				dr.GatewayVectorDatabase = append(dr.GatewayVectorDatabase, NeighborGatewayVector{})
				copy(dr.GatewayVectorDatabase[i:], dr.GatewayVectorDatabase[i+1:])
				dr.GatewayVectorDatabase[i-1] = NeighborGatewayVector{
					Sequence: sequence,
				}
				for j, val := range vector {
					dr.GatewayVectorDatabase[i-1].Vector[j] = val
				}
			}
		}
	} else {
		tmp := NeighborGatewayVector{
			Sequence: sequence}
		for j, val := range vector {
			tmp.Vector[j] = val
		}
		dr.GatewayVectorDatabase = append(dr.GatewayVectorDatabase, tmp)
	}
}

// 802.1ax-2014 9.3.4.4
func extractGatewayConversationID() {

}

// 802.1ax-2014 9.3.4.4
func extractPortConversationID() {

}
