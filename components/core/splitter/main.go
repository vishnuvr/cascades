package main

import (
	"flag"
	"fmt"
	zmq "github.com/alecthomas/gozmq"
	"github.com/cascades-fbp/cascades/components/utils"
	"github.com/cascades-fbp/cascades/runtime"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var (
	// Flags
	inputEndpoint  = flag.String("port.in", "", "Component's input port endpoint")
	outputEndpoint = flag.String("port.out", "", "Component's output port #1 endpoint")
	jsonFlag       = flag.Bool("json", false, "Print component documentation in JSON")
	debug          = flag.Bool("debug", false, "Enable debug mode")

	// Internal
	context      *zmq.Context
	outPortArray []*zmq.Socket
	inPort, port *zmq.Socket
	err          error
)

func validateArgs() {
	if *inputEndpoint == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *outputEndpoint == "" {
		flag.Usage()
		os.Exit(1)
	}
}

func openPorts() {
	outports := strings.Split(*outputEndpoint, ",")
	if len(outports) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	context, err = zmq.NewContext()
	utils.AssertError(err)

	inPort, err = utils.CreateInputPort(context, *inputEndpoint)
	utils.AssertError(err)

	outPortArray = []*zmq.Socket{}
	for i, endpoint := range outports {
		endpoint = strings.TrimSpace(endpoint)
		log.Printf("Connecting OUT[%v]=%s", i, endpoint)
		port, err = utils.CreateOutputPort(context, endpoint)
		outPortArray = append(outPortArray, port)
	}
}

func closePorts() {
	inPort.Close()
	for _, port = range outPortArray {
		port.Close()
	}
	context.Close()
}

func main() {
	flag.Parse()

	if *jsonFlag {
		doc, _ := registryEntry.JSON()
		fmt.Println(string(doc))
		os.Exit(0)
	}

	log.SetFlags(0)
	if *debug {
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	validateArgs()

	openPorts()
	defer closePorts()

	ch := utils.HandleInterruption()
	err = runtime.SetupShutdownByDisconnect(context, inPort, "splitter.in", ch)
	utils.AssertError(err)

	log.Println("Started...")
	for {
		ip, err := inPort.RecvMultipart(0)
		if err != nil {
			log.Println("Error receiving message:", err.Error())
			continue
		}
		if !runtime.IsValidIP(ip) {
			log.Println("Received invalid IP")
			continue
		}
		for _, port = range outPortArray {
			port.SendMultipart(ip, 0)
		}
	}
}
