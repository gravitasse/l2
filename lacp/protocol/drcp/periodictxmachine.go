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
// 802.1ax-2014 Section 9.4.15 DRCPDU Periodic Transmission machine
// rxmachine.go
package drcp

import (
	"github.com/google/gopacket/layers"
	"l2/lacp/protocol/utils"
	"sort"
	"time"
)

const PtxMachineModuleStr = "DRCP PTX Machine"

// drxm States
const (
	PtxmStateNone = iota + 1
	PtxmStateNoPeriodic
	PtxmStateFastPeriodic
	PtxmStateSlowPeriodic
	PtxmStatePeriodicTx
)

var PtxmStateStrMap map[fsm.State]string

func PtxMachineStrStateMapCreate() {
	PtxmStateStrMap = make(map[fsm.State]string)
	PtxmStateStrMap[PtxmStateNone] = "None"
	PtxmStateStrMap[PtxmStateNoPeriodic] = "No Periodic"
	PtxmStateStrMap[PtxmStateFastPeriodic] = "Fast Periodic"
	PtxmStateStrMap[PtxmStateSlowPeriodic] = "Slow Periodic"
	PtxmStateStrMap[PtxmStatePeriodicTx] = "Periodic Tx"
}

// rxm events
const (
	PtxmEventBegin = iota + 1
	PtxmEventNotIPPPortEnabled
	PtxmEventUnconditionalFallThrough
	PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualLongTimeout
	PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualShortTimeout
	PtxmEventDRCPPeriodicTimerExpired
)

// PtxMachine holds FSM and current State
// and event channels for State transitions
type PtxMachine struct {
	// for debugging
	PreviousState fsm.State

	Machine *fsm.Machine

	p *DRCPIpp

	// timer interval
	periodicTimerInterval time.Duration

	// timers
	periodicTimer *time.Timer

	// machine specific events
	PtxmEvents chan utils.MachineEvent
}

func (ptxm *PtxMachine) PrevState() fsm.State { return ptxm.PreviousState }

// PrevStateSet will set the previous State
func (ptxm *PtxMachine) PrevStateSet(s fsm.State) { ptxm.PreviousState = s }

// Stop should clean up all resources
func (ptxm *PtxMachine) Stop() {
	ptxm.PeriodicTimerStop()

	close(ptxm.PtxmEvents)

}

// NewDrcpPTxMachine will create a new instance of the LacpRxMachine
func NewDrcpPTxMachine(port *DRCPIpp) *PtxMachine {
	ptxm := &PtxMachine{
		p:             port,
		PreviousState: PtxmStateNone,
		RxmEvents:     make(chan MachineEvent, 10),
	}

	port.PtxMachineFsm = ptxm

	// create then stop
	ptxm.PeriodicTimerStart()
	ptxm.PeriodicTimerStop()

	return ptxm
}

// A helpful function that lets us apply arbitrary rulesets to this
// instances State machine without reallocating the machine.
func (ptxm *PtxMachine) Apply(r *fsm.Ruleset) *fsm.Machine {
	if ptxm.Machine == nil {
		ptxm.Machine = &fsm.Machine{}
	}

	// Assign the ruleset to be used for this machine
	ptxm.Machine.Rules = r
	ptxm.Machine.Curr = &utils.StateEvent{
		StrStateMap: PtxmStateStrMap,
		LogEna:      false,
		Logger:      ptxm.DrcpPtxmLog,
		Owner:       PtxMachineModuleStr,
	}

	return ptxm.Machine
}

// DrcpPtxMachineNoPeriodic function to be called after
// State transition to NO_PERIODIC
func (ptxm *PtxMachine) DrcpPtxMachineNoPeriodic(m fsm.Machine, data interface{}) fsm.State {
	prxm.PeriodicTimerStop()
	// next State
	return PtxmStateNoPeriodic
}

// DrcpPtxMachineFastPeriodic function to be called after
// State transition to FAST_PERIODIC
func (ptxm *PtxMachine) DrcpPtxMachineFastPeriodic(m fsm.Machine, data interface{}) fsm.State {
	prxm.PeriodicTimerIntervalSet(DrniFastPeriodicTime)
	prxm.PeriodicTimerStart()
	// next State
	return PtxmStateFastPeriodic
}

// DrcpPtxMachineSlowPeriodic function to be called after
// State transition to SLOW_PERIODIC
func (ptxm *PtxMachine) DrcpPtxMachineSlowPeriodic(m fsm.Machine, data interface{}) fsm.State {
	prxm.PeriodicTimerIntervalSet(DrniSlowPeriodictime)
	prxm.PeriodicTimerStart()
	// next State
	return PtxmStateSlowPeriodic
}

// DrcpPtxMachinePeriodicTx function to be called after
// State transition to PERIODIC_TX
func (ptxm *PtxMachine) DrcpPtxMachinePeriodicTx(m fsm.Machine, data interface{}) fsm.State {

	p := ptxm.p

	defer ptxm.NotifyNTTDRCPUDChange(p.NTTDRCPDU, true)
	p.NTTDRCPDU = true

	// next State
	return PtxmStatePeriodicTx
}

func DrcpRxMachineFSMBuild(p *DRCPIpp) *LacpRxMachine {

	PtxMachineStrStateMapCreate()

	rules := fsm.Ruleset{}

	// Instantiate a new LacpRxMachine
	// Initial State will be a psuedo State known as "begin" so that
	// we can transition to the initalize State
	ptxm := NewDrcpPTxMachine(p)

	//BEGIN -> NO PERIODIC
	rules.AddRule(PtxmStateNone, PtxmEventBegin, ptxm.DrcpPtxMachineNoPeriodic)
	rules.AddRule(PtxmStateNoPeriodic, PtxmEventBegin, ptxm.DrcpPtxMachineNoPeriodic)
	rules.AddRule(PtxmStateFastPeriodic, PtxmEventBegin, ptxm.DrcpPtxMachineNoPeriodic)
	rules.AddRule(PtxmStateSlowPeriodic, PtxmEventBegin, ptxm.DrcpPtxMachineNoPeriodic)
	rules.AddRule(PtxmStatePeriodicTx, PtxmEventBegin, ptxm.DrcpPtxMachineNoPeriodic)

	// NOT IPP PORT ENABLED  > NO PERIODIC
	rules.AddRule(PtxmStateNone, PtxmEventNotIPPPortEnabled, ptxm.DrcpPtxMachineNoPeriodic)
	rules.AddRule(PtxmStateNoPeriodic, PtxmEventNotIPPPortEnabled, ptxm.DrcpPtxMachineNoPeriodic)
	rules.AddRule(PtxmStateFastPeriodic, PtxmEventNotIPPPortEnabled, ptxm.DrcpPtxMachineNoPeriodic)
	rules.AddRule(PtxmStateSlowPeriodic, PtxmEventNotIPPPortEnabled, ptxm.DrcpPtxMachineNoPeriodic)
	rules.AddRule(PtxmStatePeriodicTx, PtxmEventNotIPPPortEnabled, ptxm.DrcpPtxMachineNoPeriodic)

	// Unconditional  > FAST PERIODIC
	rules.AddRule(PtxmStateNoPeriodic, PtxmEventUnconditionalFallThrough, ptxm.DrcpPtxMachineFastPeriodic)

	// IPP PORT ENABLED AND DRCP ENABLED -> EXPIRED
	rules.AddRule(PtxmStateFastPeriodic, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualLongTimeout, ptxm.DrcpPtxMachineSlowPeriodic)

	// PERIODIC TIMER EXPIRED -> PERIODIC TX
	rules.AddRule(PtxmStateFastPeriodic, PtxmEventDRCPPeriodicTimerExpired, ptxm.DrcpPtxMachinePeriodicTx)
	rules.AddRule(PtxmStateSlowPeriodic, PtxmEventDRCPPeriodicTimerExpired, ptxm.DrcpPtxMachinePeriodicTx)

	// DRF NEIGHBOR OPER DRCP STATE == SHORT TIMEOUT -> FAST PERIODIC OR PERIODIC TX
	rules.AddRule(PtxmStatePeriodicTx, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualShortTimeout, ptxm.DrcpPtxMachineFastPeriodic)
	rules.AddRule(PtxmStateSlowPeriodic, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualShortTimeout, ptxm.DrcpPtxMachinePeriodicTx)

	// DRF NEIGHBOR OPER DRCP STATE == LONG TIMEOUT -> SLOW PERIODIC
	rules.AddRule(PtxmStatePeriodicTx, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualLongTimeout, ptxm.DrcpPtxMachineSlowPeriodic)
	rules.AddRule(PtxmStateFastPeriodic, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualLongTimeout, ptxm.DrcpPtxMachineSlowPeriodic)

	// Create a new FSM and apply the rules
	rxm.Apply(&rules)

	return rxm
}

// DrcpRxMachineMain:  802.1ax-2014 Figure 9-23
// Creation of Rx State Machine State transitions and callbacks
// and create go routine to pend on events
func (p *DRCPIpp) DrcpRxMachineMain() {

	// Build the State machine for Lacp Receive Machine according to
	// 802.1ax Section 6.4.12 Receive Machine
	rxm := DrcpRxMachineFSMBuild(p)
	p.wg.Add(1)

	// set the inital State
	rxm.Machine.Start(rxm.PrevState())

	// lets create a go routing which will wait for the specific events
	// that the RxMachine should handle.
	go func(m *LacpRxMachine) {
		m.LacpRxmLog("Machine Start")
		defer m.p.wg.Done()
		for {
			select {
			case <-m.currentWhileTimer.C:

				m.Machine.ProcessEvent(PtxMachineModuleStr, PtxmEventDRCPPeriodicTimerExpired, nil)

				if m.Machine.Curr.CurrentState() == PtxmStatePeriodicTx {
					if p.DRFNeighborOperDRCPState.GetState(layers.DRCPStateDRCPTimeout) == layers.DRCPLongTimeout {
						m.Machine.ProcessEvent(PtxMachineModuleStr, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualLongTimeout, nil)
					} else {
						m.Machine.ProcessEvent(PtxMachineModuleStr, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualShortTimeout, nil)
					}
				}

			case event, ok := <-m.RxmEvents:
				if ok {
					rv := m.Machine.ProcessEvent(event.src, event.e, nil)
					if rv == nil {
						p := m.p
						/* continue State transition */
						if m.Machine.Curr.CurrentState() == PtxmStateNoPeriodic {
							rv = m.Machine.ProcessEvent(RxMachineModuleStr, LacpRxmEventUnconditionalFallthrough, nil)
						}
						// post processing
						if rv == nil {
							if m.Machine.Curr.CurrentState() == PtxmStateFastPeriodic &&
								p.DRFNeighborOperDRCPState.GetState(layers.DRCPStateDRCPTimeout) == layers.DRCPLongTimeout {
								rv = m.Machine.ProcessEvent(RxMachineModuleStr, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualLongTimeout, nil)
							}
							if rv == nil &&
								m.Machine.Curr.CurrentState() == PtxmStateSlowPeriodic &&
								p.DRFNeighborOperDRCPState.GetState(layers.DRCPStateDRCPTimeout) == layers.DRCPShortTimeout {
								rv = m.Machine.ProcessEvent(RxMachineModuleStr, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualShortTimeout, nil)
							}
							if rv == nil &&
								m.Machine.Curr.CurrentState() == PtxmStatePeriodicTx {
								if p.DRFNeighborOperDRCPState.GetState(layers.DRCPStateDRCPTimeout) == layers.DRCPLongTimeout {
									rv = m.Machine.ProcessEvent(RxMachineModuleStr, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualLongTimeout, nil)
								} else {
									rv = m.Machine.ProcessEvent(RxMachineModuleStr, PtxmEventDRFNeighborOPerDRCPStateTimeoutEqualShortTimeout, nil)
								}
							}
						}
					}

					if rv != nil {
						m.DrcpPtxmLog(strings.Join([]string{error.Error(rv), event.src, RxmStateStrMap[m.Machine.Curr.CurrentState()], strconv.Itoa(int(event.e))}, ":"))
					}
				}

				// respond to caller if necessary so that we don't have a deadlock
				if event.ResponseChan != nil {
					utils.SendResponse(PtxMachineModuleStr, event.ResponseChan)
				}
				if !ok {
					m.DrcpPtxmLog("Machine End")
					return
				}
			}
		}
	}(rxm)
}

// NotifyNTTDRCPUDChange
func (ptxm PtxMachine) NotifyNTTDRCPUDChange(oldval, newval bool) {
	p := ptxm.p
	if oldval != newval &&
		p.NTTDRCPDU {
		// TODO send event to other state machine
	}
}
