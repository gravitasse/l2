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

package lacp

import (
	"fmt"
	"testing"
)

func TestCreateDeleteLaAggregatorNoMembers(t *testing.T) {

	// must be called to initialize the global
	sysId := LacpSystem{Actor_System_priority: 128,
		actor_System: [6]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}}

	LacpSysGlobalInfoInit(sysId)

	aconf := &LaAggConfig{
		Name: "agg2000",
		Mac:  [6]uint8{0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
		Id:   2000,
		Key:  50,
		Lacp: LacpConfigInfo{Interval: LacpSlowPeriodicTime,
			Mode:           LacpModeActive,
			SystemIdMac:    "00:01:02:03:04:05",
			SystemPriority: 128},
	}

	// lets create an aggregator
	agg := NewLaAggregator(aconf)
	agg.DeleteLaAgg()

	for _, sgi := range LacpSysGlobalInfoGet() {
		if len(sgi.AggList) > 0 || len(sgi.AggMap) > 0 {
			t.Error("System Agg List or Map is not empty", sgi.AggList, sgi.AggMap)
		}
		if len(sgi.PortList) > 0 || len(sgi.PortMap) > 0 {
			t.Error("System Port List or Map is not empty", sgi.PortList, sgi.PortMap)
		}
	}
}

func TestCreateDeleteLaAggregatorWithMembers(t *testing.T) {

	// must be called to initialize the global
	sysId := LacpSystem{Actor_System_priority: 128,
		actor_System: [6]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}}

	LacpSysGlobalInfoInit(sysId)

	aconf := &LaAggConfig{
		Name: "agg2000",
		Mac:  [6]uint8{0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
		Id:   2000,
		Key:  50,
		Lacp: LacpConfigInfo{Interval: LacpSlowPeriodicTime,
			Mode:           LacpModeActive,
			SystemIdMac:    "00:01:02:03:04:05",
			SystemPriority: 128},
		LagMembers: []uint16{1, 2, 3, 4},
	}

	// lets create a port and start the machines
	agg := NewLaAggregator(aconf)

	// Delete the port and agg
	agg.DeleteLaAgg()
	for _, sgi := range LacpSysGlobalInfoGet() {
		if len(sgi.AggList) > 0 || len(sgi.AggMap) > 0 {
			t.Error("System Agg List or Map is not empty", sgi.SysKey, sgi.AggList, sgi.AggMap)
		}
		if len(sgi.PortList) > 0 || len(sgi.PortMap) > 0 {
			t.Error("System Port List or Map is not empty", sgi.SysKey, sgi.PortList, sgi.PortMap)
		}
	}
}

func TestCreateDeleteFindByAggName(t *testing.T) {

	// must be called to initialize the global
	sysId := LacpSystem{Actor_System_priority: 128,
		actor_System: [6]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}}

	LacpSysGlobalInfoInit(sysId)

	aconf := &LaAggConfig{
		Name: "agg2000",
		Mac:  [6]uint8{0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
		Id:   2000,
		Key:  50,
		Lacp: LacpConfigInfo{Interval: LacpSlowPeriodicTime,
			Mode:           LacpModeActive,
			SystemIdMac:    "00:01:02:03:04:05",
			SystemPriority: 128},
		LagMembers: []uint16{1, 2, 3, 4},
	}

	var na *LaAggregator
	// valid name, but agg not created yet
	if LaFindAggByName("agg2000", &na) {
		t.Error("Error found aggregator by name")
	}

	// lets create a port and start the machines
	agg := NewLaAggregator(aconf)

	// valid name after agg created
	if !LaFindAggByName("agg2000", &na) {
		t.Error("Error did not find aggregator by name")
	}

	// invalid name
	if LaFindAggByName("agg2001", &na) {
		t.Error("Error found aggregator by invalid name")
	}

	// Delete the port and agg
	agg.DeleteLaAgg()
	for _, sgi := range LacpSysGlobalInfoGet() {
		if len(sgi.AggList) > 0 || len(sgi.AggMap) > 0 {
			t.Error("System Agg List or Map is not empty", sgi.SysKey, sgi.AggList, sgi.AggMap)
		}
		if len(sgi.PortList) > 0 || len(sgi.PortMap) > 0 {
			t.Error("System Port List or Map is not empty", sgi.SysKey, sgi.PortList, sgi.PortMap)
		}
	}

	// valid name after delete
	if LaFindAggByName("agg2000", &na) {
		t.Error("Error found aggregator by name")
	}

}

func TestCreateDeleteFindById(t *testing.T) {

	// must be called to initialize the global
	sysId := LacpSystem{Actor_System_priority: 128,
		actor_System: [6]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}}

	LacpSysGlobalInfoInit(sysId)

	aconf := &LaAggConfig{
		Name: "agg2000",
		Mac:  [6]uint8{0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
		Id:   2000,
		Key:  50,
		Lacp: LacpConfigInfo{Interval: LacpSlowPeriodicTime,
			Mode:           LacpModeActive,
			SystemIdMac:    "00:01:02:03:04:05",
			SystemPriority: 128},
		LagMembers: []uint16{1, 2, 3, 4},
	}

	var na *LaAggregator
	// valid id but agg not created yet
	if LaFindAggById(2000, &na) {
		t.Error("Error found aggregator by id")
	}

	// lets create a port and start the machines
	agg := NewLaAggregator(aconf)

	// valid id
	if !LaFindAggById(2000, &na) {
		t.Error("Error did not find aggregator by id")
	}

	// invalid id
	if LaFindAggById(2001, &na) {
		t.Error("Error found aggregator with bad id")
	}

	// Delete the port and agg
	agg.DeleteLaAgg()
	for _, sgi := range LacpSysGlobalInfoGet() {
		if len(sgi.AggList) > 0 || len(sgi.AggMap) > 0 {
			t.Error("System Agg List or Map is not empty", sgi.SysKey, sgi.AggList, sgi.AggMap)
		}
		if len(sgi.PortList) > 0 || len(sgi.PortMap) > 0 {
			t.Error("System Port List or Map is not empty", sgi.SysKey, sgi.PortList, sgi.PortMap)
		}
	}

	// valid id but should be removed from list
	if LaFindAggById(2000, &na) {
		t.Error("Error found aggregator by name")
	}
}

func TestCreateDeleteFindByKey(t *testing.T) {

	// must be called to initialize the global
	sysId := LacpSystem{Actor_System_priority: 128,
		actor_System: [6]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}}

	LacpSysGlobalInfoInit(sysId)

	aconf := &LaAggConfig{
		Name: "agg2000",
		Mac:  [6]uint8{0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
		Id:   2000,
		Key:  50,
		Lacp: LacpConfigInfo{Interval: LacpSlowPeriodicTime,
			Mode:           LacpModeActive,
			SystemIdMac:    "00:01:02:03:04:05",
			SystemPriority: 128},
		LagMembers: []uint16{1, 2, 3, 4},
	}

	var na *LaAggregator
	// valid id but agg not created yet
	if LaFindAggById(50, &na) {
		t.Error("Error found aggregator by key")
	}

	// lets create a port and start the machines
	agg := NewLaAggregator(aconf)

	// valid id after create
	if !LaFindAggByKey(50, &na) {
		t.Error("Error did not find aggregator by key")
	}

	// invalid id
	if LaFindAggByKey(51, &na) {
		t.Error("Error found aggregator by invalid key")
	}

	// Delete the port and agg
	agg.DeleteLaAgg()
	for _, sgi := range LacpSysGlobalInfoGet() {
		if len(sgi.AggList) > 0 || len(sgi.AggMap) > 0 {
			t.Error("System Agg List or Map is not empty", sgi.SysKey, sgi.AggList, sgi.AggMap)
		}
		if len(sgi.PortList) > 0 || len(sgi.PortMap) > 0 {
			t.Error("System Port List or Map is not empty", sgi.SysKey, sgi.PortList, sgi.PortMap)
		}
	}

	// valid id but agg has been deleted
	if LaFindAggByKey(50, &na) {
		t.Error("Error found aggregator by deleted key")
	}
}

func TestCreateDeleteFindLacpPortMember(t *testing.T) {

	// must be called to initialize the global
	sysId := LacpSystem{Actor_System_priority: 128,
		actor_System: [6]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}}

	LacpSysGlobalInfoInit(sysId)

	aconf := &LaAggConfig{
		Name: "agg2000",
		Mac:  [6]uint8{0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
		Id:   2000,
		Key:  50,
		Lacp: LacpConfigInfo{Interval: LacpSlowPeriodicTime,
			Mode:           LacpModeActive,
			SystemIdMac:    "00:01:02:03:04:05",
			SystemPriority: 128},
		LagMembers: []uint16{1, 2, 3, 4},
	}

	var na *LaAggregator
	if LaFindAggById(50, &na) {
		t.Error("Error found aggregator by name")
	}

	// lets create a port and start the machines
	agg := NewLaAggregator(aconf)

	if !LaFindAggByKey(50, &na) {
		t.Error("Error did not find aggregator by name")
	}

	// good key, good port
	if !LaAggPortNumListPortIdExist(50, 3) {
		t.Error("Error did not find Port member")
	}

	// bad key, good port
	if LaAggPortNumListPortIdExist(51, 3) {
		t.Error("Error found Port member with bad key")
	}

	// bad key, bad port
	if LaAggPortNumListPortIdExist(51, 3) {
		t.Error("Error found Port member with bad key")
	}

	// Delete the port and agg
	agg.DeleteLaAgg()
	for _, sgi := range LacpSysGlobalInfoGet() {
		if len(sgi.AggList) > 0 || len(sgi.AggMap) > 0 {
			t.Error("System Agg List or Map is not empty", sgi.SysKey, sgi.AggList, sgi.AggMap)
		}
		if len(sgi.PortList) > 0 || len(sgi.PortMap) > 0 {
			t.Error("System Port List or Map is not empty", sgi.SysKey, sgi.PortList, sgi.PortMap)
		}
	}

	if LaFindAggByKey(50, &na) {
		t.Error("Error found aggregator by name")
	}
}

func TestDuplicateAdd(t *testing.T) {

	// must be called to initialize the global
	sysId := LacpSystem{Actor_System_priority: 128,
		actor_System: [6]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}}

	LacpSysGlobalInfoInit(sysId)

	aconf := &LaAggConfig{
		Name: "agg2000",
		Mac:  [6]uint8{0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
		Id:   2000,
		Key:  50,
		Lacp: LacpConfigInfo{Interval: LacpSlowPeriodicTime,
			Mode:           LacpModeActive,
			SystemIdMac:    "00:01:02:03:04:05",
			SystemPriority: 128},
		LagMembers: []uint16{1, 2, 3, 4},
	}

	var na *LaAggregator
	if LaFindAggById(50, &na) {
		t.Error("Error found aggregator by name")
	}

	// lets create a port and start the machines
	agg1 := NewLaAggregator(aconf)
	if agg1 == nil {
		t.Error("Error aggregator was not created")
	}
	agg2 := NewLaAggregator(aconf)
	if agg2 != nil {
		t.Error("Error Aggregator should have failed as tried to create a duplicate Agg")
	}

	agg1.DeleteLaAgg()

}

// Worst case is usually a single port lag and one per port
// so lets test a 128 port switch
func TestScaleAggCreate(t *testing.T) {

	// must be called to initialize the global
	sysId := LacpSystem{Actor_System_priority: 128,
		actor_System: [6]uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}}

	LacpSysGlobalInfoInit(sysId)

	for id := 1; id < 129; id++ {

		aconf := &LaAggConfig{
			Name: fmt.Sprintf("agg%d", 1000+id),
			Mac:  [6]uint8{0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
			Id:   1000 + id,
			Key:  50 + uint16(id),
			Lacp: LacpConfigInfo{Interval: LacpSlowPeriodicTime,
				Mode:           LacpModeActive,
				SystemIdMac:    "00:01:02:03:04:05",
				SystemPriority: 128},
			LagMembers: []uint16{uint16(id)},
		}

		var na *LaAggregator
		if LaFindAggById(50, &na) {
			t.Error("Error found aggregator by name")
		}

		// lets create a port and start the machines
		NewLaAggregator(aconf)

		if !LaFindAggById(1000+id, &na) {
			t.Error("Error did not find aggregator by name")
		}

		// good key, good port
		if !LaAggPortNumListPortIdExist(50+uint16(id), uint16(id)) {
			t.Error("Error did not find Port member")
		}
	}

	var agg *LaAggregator
	var aggCount int

	// lets test iterating over the agg list
	for LaGetAggNext(&agg) {
		aggCount += 1
	}

	if aggCount != 128 {
		t.Error("Was not able to find 128 agg groups found", aggCount)
	}

	sgi := LacpSysGlobalInfoByIdGet(sysId)
	if len(sgi.AggList) != 128 {
		t.Error("Failed to add 128 lag groups to agg map", len(sgi.AggList))
	}
	if len(sgi.AggMap) != 128 {
		t.Error("Failed to add 128 lag groups to agg map", len(sgi.AggMap))
	}

	for id := 1; id < 129; id++ {
		var agg *LaAggregator
		if LaFindAggById(1000+id, &agg) {
			// Delete the port and agg
			agg.DeleteLaAgg()
		}
	}
	for _, sgi := range LacpSysGlobalInfoGet() {
		if len(sgi.AggList) > 0 || len(sgi.AggMap) > 0 {
			t.Error("System Agg List or Map is not empty", sgi.SysKey, sgi.AggList, sgi.AggMap)
		}
		if len(sgi.PortList) > 0 || len(sgi.PortMap) > 0 {
			t.Error("System Port List or Map is not empty", sgi.SysKey, sgi.PortList, sgi.PortMap)
		}
	}
}
