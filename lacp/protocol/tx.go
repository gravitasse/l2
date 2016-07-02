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

// tx
package lacp

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"net"
)

// bridge will simulate communication between two channels
type SimulationBridge struct {
	port1       uint16
	port2       uint16
	rxLacpPort1 chan gopacket.Packet
	rxLacpPort2 chan gopacket.Packet
	rxLampPort1 chan gopacket.Packet
	rxLampPort2 chan gopacket.Packet
}

func (bridge *SimulationBridge) TxViaGoChannel(port uint16, pdu interface{}) {

	var p *LaAggPort
	if LaFindPortById(port, &p) {

		// Set up all the layers' fields we can.
		eth := layers.Ethernet{
			SrcMAC:       net.HardwareAddr{0x00, uint8(p.PortNum & 0xff), 0x00, 0x01, 0x01, 0x01},
			DstMAC:       layers.SlowProtocolDMAC,
			EthernetType: layers.EthernetTypeSlowProtocol,
		}
		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			FixLengths:       true,
			ComputeChecksums: true,
		}

		switch pdu.(type) {
		case *layers.LACP:
			slow := layers.SlowProtocol{
				SubType: layers.SlowProtocolTypeLACP,
			}
			lacp := pdu.(*layers.LACP)

			gopacket.SerializeLayers(buf, opts, &eth, &slow, lacp)

		case *layers.LAMP:
			slow := layers.SlowProtocol{
				SubType: layers.SlowProtocolTypeLAMP,
			}
			lamp := pdu.(*layers.LAMP)
			gopacket.SerializeLayers(buf, opts, &eth, &slow, lamp)
		}

		pkt := gopacket.NewPacket(buf.Bytes(), layers.LinkTypeEthernet, gopacket.Default)

		if port != bridge.port1 && bridge.rxLacpPort1 != nil {
			//fmt.Println("TX channel: Tx From port", port, "bridge Port Rx", bridge.port1)
			//fmt.Println("TX:", pkt)
			bridge.rxLacpPort1 <- pkt
		} else if bridge.rxLacpPort2 != nil {
			//fmt.Println("TX channel: Tx From port", port, "bridge Port Rx", bridge.port2)
			bridge.rxLacpPort2 <- pkt
		}
	} else {
		fmt.Println("Unable to find port in tx")
	}
}

func TxViaLinuxIf(port uint16, pdu interface{}) {
	var p *LaAggPort
	if LaFindPortById(port, &p) {

		txIface, err := net.InterfaceByName(p.IntfNum)

		if err == nil {
			// conver the packet to a go packet
			// Set up all the layers' fields we can.
			eth := layers.Ethernet{
				SrcMAC:       txIface.HardwareAddr,
				DstMAC:       layers.SlowProtocolDMAC,
				EthernetType: layers.EthernetTypeSlowProtocol,
			}

			// Set up buffer and options for serialization.
			buf := gopacket.NewSerializeBuffer()
			opts := gopacket.SerializeOptions{
				FixLengths:       true,
				ComputeChecksums: true,
			}

			switch pdu.(type) {
			case *layers.LACP:
				slow := layers.SlowProtocol{
					SubType: layers.SlowProtocolTypeLACP,
				}
				lacp := pdu.(*layers.LACP)
				gopacket.SerializeLayers(buf, opts, &eth, &slow, lacp)

			case *layers.LAMP:
				slow := layers.SlowProtocol{
					SubType: layers.SlowProtocolTypeLAMP,
				}
				lamp := pdu.(*layers.LAMP)
				gopacket.SerializeLayers(buf, opts, &eth, &slow, lamp)
			}

			// Send one packet for every address.
			if err := p.handle.WritePacketData(buf.Bytes()); err != nil {
				p.LacpDebug.logger.Info(fmt.Sprintf("%s\n", err))
			}
		} else {
			fmt.Println("ERROR could not find interface", p.IntfNum, err)
		}
	} else {
		fmt.Println("Unable to find port", port)
	}
}
