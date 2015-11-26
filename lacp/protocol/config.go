// config
package lacp

import (
	"fmt"
	//"sync"
	"net"
	"time"
)

const (
	LaAggTypeLACP = iota + 1
	LaAggTypeSTATIC
)

const PortConfigModuleStr = "Port Config"

type LaAggConfig struct {
	// Aggregator name
	Name string
	// Aggregator_MAC_address
	Mac [6]uint8
	// Aggregator_Identifier
	Id int
	// Actor_Admin_Aggregator_Key
	Key uint16
	// Aggregator Type, LACP or STATIC
	Type uint32
	// Minimum number of links
	MinLinks uint16
	// LAG_ports
	LagMembers []uint16

	// system to attach this agg to
	SysId net.HardwareAddr

	// TODO hash config
}

type LaAggPortConfig struct {

	// Actor_Port_Number
	Id uint16
	// Actor_Port_Priority
	Prio uint16
	// Actor Admin Key
	Key uint16
	// Actor Oper Key
	//OperKey uint16
	// Actor_Port_Aggregator_Identifier
	AggId int

	// Admin Enable/Disable
	Enable bool

	// lacp mode On/Active/Passive
	Mode int

	// lacp timeout SHORT/LONG
	Timeout time.Duration

	// Port capabilities and attributes
	Properties PortProperties

	// system to attach this agg to
	SysId net.HardwareAddr

	// Linux If
	TraceEna bool
	IntfId   string
}

func CreateLaAgg(agg *LaAggConfig) {

	//var wg sync.WaitGroup

	a := NewLaAggregator(agg)
	//fmt.Printf("%#v\n", a)

	/*
		// two methods for creating ports after CreateLaAgg is created
		// 1) PortNumList is populated
		// 2) find Key's that match
		for _, pId := range a.PortNumList {
			wg.Add(1)
			go func(pId uint16) {
				var p *LaAggPort
				defer wg.Done()
				if LaFindPortById(pId, &p) && p.aggSelected == LacpAggUnSelected {
					// if aggregation has been provided then lets kick off the process
					p.checkConfigForSelection()
				}
			}(pId)
		}

		wg.Wait()
	*/
	index := 0
	var p *LaAggPort
	if mac, err := net.ParseMAC(a.Config.SystemIdMac); err == nil {
		if sgi := LacpSysGlobalInfoGet(mac); sgi != nil {
			for index != -1 {
				if LaFindPortByKey(a.actorAdminKey, &index, &p) {
					if p.aggSelected == LacpAggUnSelected {
						AddLaAggPortToAgg(a.aggId, p.portNum)
					}
				} else {
					break
				}
			}
		}
	}
}

func DeleteLaAgg(Id int) {
	var a *LaAggregator
	if LaFindAggById(Id, &a) {

		for _, pId := range a.PortNumList {
			DeleteLaAggPortFromAgg(Id, pId)
			DeleteLaAggPort(pId)
		}

		a.aggId = 0
		a.actorAdminKey = 0
		a.partnerSystemId = [6]uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		a.ready = false
	}
}

func CreateLaAggPort(port *LaAggPortConfig) {
	var pTmp *LaAggPort

	// sanity check that port does not exist already
	if !LaFindPortById(port.Id, &pTmp) {
		p := NewLaAggPort(port)
		//p.LaPortLog(fmt.Sprintf("Creating LaAggPort %d", port.Id))
		//fmt.Printf("%#v\n", p)

		// Is lacp enabled or not
		if port.Mode != LacpModeOn {
			p.lacpEnabled = true
			// make the port aggregatable
			LacpStateSet(&p.actorAdmin.state, LacpStateAggregationBit)
			// set the activity state
			if port.Mode == LacpModeActive {
				LacpStateSet(&p.actorAdmin.state, LacpStateActivityBit)
			} else {
				LacpStateClear(&p.actorAdmin.state, LacpStateActivityBit)
			}
		} else {
			// port is not aggregatible
			LacpStateClear(&p.actorAdmin.state, LacpStateAggregationBit)
			LacpStateClear(&p.actorAdmin.state, LacpStateActivityBit)
			p.lacpEnabled = false
		}

		if port.Timeout == LacpShortTimeoutTime {
			LacpStateSet(&p.actorAdmin.state, LacpStateTimeoutBit)
		} else {
			LacpStateClear(&p.actorAdmin.state, LacpStateTimeoutBit)
		}

		// lets start all the state machines
		p.BEGIN(false)

		// TODO: need logic to check link status
		p.linkOperStatus = true

		if p.linkOperStatus && port.Enable {

			if p.key != 0 {
				var a *LaAggregator
				if LaFindAggByKey(p.key, &a) {
					p.LaPortLog("Found Agg by Key, attaching port to agg")
					// If the agg is defined lets add port to
					AddLaAggPortToAgg(a.aggId, p.portNum)
				}
			}

			// if port is enabled and lacp is enabled
			p.LaAggPortEnabled()

		}
	} else {
		fmt.Println("CONF: ERROR PORT ALREADY EXISTS")
	}
}

func DeleteLaAggPort(pId uint16) {
	var p *LaAggPort
	if LaFindPortById(pId, &p) {
		if LaAggPortNumListPortIdExist(p.AggId, pId) {
			fmt.Println("CONF: ERROR Must detach p", pId, "from agg", p.AggId, "before deletion")
			return
		}
		if p.portEnabled {
			DisableLaAggPort(p.portNum)
		}

		p.DelLaAggPort()
	}
}

func DisableLaAggPort(pId uint16) {
	var p *LaAggPort

	// port exists
	// port exists in agg exists
	if LaFindPortById(pId, &p) &&
		LaAggPortNumListPortIdExist(p.AggId, pId) {
		p.LaAggPortDisable()
	}
}

func EnableLaAggPort(pId uint16) {
	var p *LaAggPort

	// port exists
	// port is unselected
	// agg exists
	if LaFindPortById(pId, &p) &&
		p.aggSelected == LacpAggUnSelected &&
		LaAggPortNumListPortIdExist(p.AggId, pId) {
		p.LaAggPortEnabled()

		// TODO: NEED METHOD to get link status
		p.linkOperStatus = true

		if p.linkOperStatus &&
			p.aggSelected == LacpAggUnSelected {
			p.checkConfigForSelection()
		}
	}
}

// SetLaAggPortLacpMode will set the various
// lacp modes - On, Active, Passive
// timeout -LacpShortTimeoutTime, LacpLongTimeoutTime, 0
func SetLaAggPortLacpMode(pId uint16, mode int, timeout time.Duration) {

	var p *LaAggPort

	// port exists
	// port is unselected
	// agg exists
	if LaFindPortById(pId, &p) {
		prevMode := LacpModeGet(p.actorOper.state, p.lacpEnabled)
		p.LaPortLog(fmt.Sprintf("PrevMode", prevMode, "NewMode", mode, "timeout", timeout))
		// TODO need a way to not update the timer cause users may not care
		// to set it and may want to just leave it alone
		if timeout != 0 {
			// update the periodic timer
			if LacpStateIsSet(p.actorOper.state, LacpStateTimeoutBit) &&
				timeout == LacpLongTimeoutTime {
				LacpStateClear(&p.actorAdmin.state, LacpStateTimeoutBit)
				LacpStateClear(&p.actorOper.state, LacpStateTimeoutBit)
			} else if !LacpStateIsSet(p.actorAdmin.state, LacpStateTimeoutBit) &&
				timeout == LacpShortTimeoutTime {
				LacpStateSet(&p.actorAdmin.state, LacpStateTimeoutBit)
				LacpStateSet(&p.actorOper.state, LacpStateTimeoutBit)
			}
		}

		// Update the transmission mode
		if mode != prevMode &&
			mode == LacpModeOn {
			p.LaAggPortLacpDisable()

			// Actor/Partner Aggregation == true
			// agg individual
			// partner admin key, port == actor admin key, port
			if p.MuxMachineFsm.Machine.Curr.CurrentState() == LacpMuxmStateDetached ||
				p.MuxMachineFsm.Machine.Curr.CurrentState() == LacpMuxmStateCDetached {
				// lets check for selection
				p.checkConfigForSelection()
			}

		} else if mode != prevMode &&
			prevMode == LacpModeOn {
			p.LaAggPortLacpEnabled(mode)
		} else if mode != prevMode {
			if mode == LacpModeActive {
				LacpStateSet(&p.actorAdmin.state, LacpStateActivityBit)
				// must also set the operational state
				LacpStateSet(&p.actorOper.state, LacpStateActivityBit)
			} else {
				LacpStateClear(&p.actorAdmin.state, LacpStateActivityBit)
				// must also set the operational state
				LacpStateClear(&p.actorOper.state, LacpStateActivityBit)
			}
		}
	}
}

func AddLaAggPortToAgg(aggId int, pId uint16) {

	var a *LaAggregator
	var p *LaAggPort

	// both add and port must have existed
	if LaFindAggById(aggId, &a) && LaFindPortById(pId, &p) &&
		p.aggSelected == LacpAggUnSelected &&
		!LaAggPortNumListPortIdExist(aggId, pId) {

		p.LaPortLog(fmt.Sprintf("Adding LaAggPort %d to LaAgg %d", pId, aggId))
		// add port to port number list
		a.PortNumList = append(a.PortNumList, p.portNum)
		// add reference to aggId
		p.AggId = aggId

		// attach the port to the aggregator
		//LacpStateSet(&p.actorAdmin.state, LacpStateAggregationBit)

		// Port is now aggregatible
		//LacpStateSet(&p.actorOper.state, LacpStateAggregationBit)

		// well obviously this should pass
		//p.checkConfigForSelection()
	}
}

func DeleteLaAggPortFromAgg(aggId int, pId uint16) {

	var a *LaAggregator
	var p *LaAggPort

	// both add and port must have existed
	if LaFindAggById(aggId, &a) && LaFindPortById(pId, &p) &&
		p.aggSelected == LacpAggSelected &&
		LaAggPortNumListPortIdExist(aggId, pId) {

		// detach the port from the agg port list
		for idx, portNum := range a.PortNumList {
			if portNum == pId {
				a.PortNumList = append(a.PortNumList[:idx], a.PortNumList[idx+1:]...)
			}
		}

		LacpStateClear(&p.actorAdmin.state, LacpStateAggregationBit)

		// if port is enabled and lacp is enabled
		p.LaAggPortDisable()

		// Port is now aggregatible
		//LacpStateClear(&p.actorOper.state, LacpStateAggregationBit)
		// inform mux machine of change of state
		// unnecessary as rx machine should set unselected to mux
		//p.checkConfigForSelection()
	}
}

func GetLaAggPortActorOperState(pId uint16) uint8 {
	var p *LaAggPort
	if LaFindPortById(pId, &p) {
		return p.actorOper.state
	}
	return 0
}

func GetLaAggPortPartnerOperState(pId uint16) uint8 {
	var p *LaAggPort
	if LaFindPortById(pId, &p) {
		return p.partnerOper.state
	}
	return 0
}
