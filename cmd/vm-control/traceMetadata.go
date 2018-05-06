package main

import (
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/Symantec/Dominator/lib/log"
	"github.com/Symantec/Dominator/lib/srpc"
	proto "github.com/Symantec/Dominator/proto/hypervisor"
)

func traceVmMetadataSubcommand(args []string, logger log.DebugLogger) {
	if err := traceVmMetadata(args[0], logger); err != nil {
		fmt.Fprintf(os.Stderr, "Error tracing VM metadata: %s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func traceVmMetadata(ipAddr string, logger log.DebugLogger) error {
	vmIP := net.ParseIP(ipAddr)
	if hypervisor, err := findHypervisor(vmIP); err != nil {
		return err
	} else {
		return traceVmMetadataOnHypervisor(hypervisor, vmIP, logger)
	}
}

func traceVmMetadataOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := srpc.DialHTTP("tcp", hypervisor, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	return doTraceMetadata(client, ipAddr, logger)
}

func maybeTraceMetadata(client *srpc.Client, ipAddr net.IP,
	logger log.Logger) error {
	if !*traceMetadata {
		return nil
	}
	return doTraceMetadata(client, ipAddr, logger)
}

func doTraceMetadata(client *srpc.Client, ipAddr net.IP,
	logger log.Logger) error {
	request := proto.TraceVmMetadataRequest{ipAddr}
	conn, err := client.Call("Hypervisor.TraceVmMetadata")
	if err != nil {
		return err
	}
	defer conn.Close()
	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)
	if err := encoder.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	var reply proto.TraceVmMetadataResponse
	if err := decoder.Decode(&reply); err != nil {
		return err
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	for {
		if line, err := conn.ReadString('\n'); err != nil {
			return err
		} else {
			if line == "\n" {
				return nil
			}
			logger.Print(line)
		}
	}
	return nil
}