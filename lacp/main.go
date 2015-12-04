// main
package main

import (
	"fmt"
	"git.apache.org/thrift.git/lib/go/thrift"
	lacp "l2/lacp/protocol"
	"l2/lacp/rpc"
	"lacpd"
	"net"
)

func main() {

	var transport thrift.TServerTransport
	var err error

	// lookup port
	configFile := "/opt/flexswitch/params/clients.json"
	port := lacp.GetClientPort(configFile, "lacpd")
	if port != 0 {
		addr := fmt.Sprintf("localhost:%d", port)
		transport, err = thrift.NewTServerSocket(addr)
		if err != nil {
			panic(fmt.Sprintf("Failed to create Socket with:", addr))
		}

		handler := rpc.NewLACPDServerHandler()
		processor := lacpd.NewLACPDServerProcessor(handler)
		transportFactory := thrift.NewTBufferedTransportFactory(8192)
		protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
		server := thrift.NewTSimpleServer4(processor, transport, transportFactory, protocolFactory)
		fmt.Println("Starting LACP Thrift daemon")

		fmt.Println("Available Interfaces for use:")
		intfs, err := net.Interfaces()
		if err == nil {
			for _, intf := range intfs {
				fmt.Println(intf)
			}
			server.Serve()
		} else {
			panic(err)
		}
		lacp.ConnectToClients(configFile)
	}
}
