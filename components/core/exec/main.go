package main

import (
	"bytes"
	"flag"
	"fmt"
	zmq "github.com/alecthomas/gozmq"
	"github.com/cascades-fbp/cascades/components/utils"
	"github.com/cascades-fbp/cascades/runtime"
	"io/ioutil"
	"log"
	"os"
)

var (
	// flags
	cmdEndpoint    = flag.String("port.cmd", "", "Component's options port endpoint")
	outputEndpoint = flag.String("port.out", "", "Component's output port endpoint")
	errorEndpoint  = flag.String("port.err", "", "Component's output port endpoint")
	jsonFlag       = flag.Bool("json", false, "Print component documentation in JSON")
	debug          = flag.Bool("debug", false, "Enable debug mode")

	// Internal
	context                   *zmq.Context
	cmdPort, outPort, errPort *zmq.Socket
	err                       error
)

func validateArgs() {
	if *cmdEndpoint == "" {
		flag.Usage()
		os.Exit(1)
	}
}

func openPorts() {
	context, err = zmq.NewContext()
	utils.AssertError(err)

	cmdPort, err = utils.CreateInputPort(context, *cmdEndpoint)
	utils.AssertError(err)

	if *outputEndpoint != "" {
		outPort, err = utils.CreateOutputPort(context, *outputEndpoint)
		utils.AssertError(err)
	}

	if *errorEndpoint != "" {
		errPort, err = utils.CreateOutputPort(context, *errorEndpoint)
		utils.AssertError(err)
	}
}

func closePorts() {
	cmdPort.Close()
	if outPort != nil {
		outPort.Close()
	}
	if errPort != nil {
		errPort.Close()
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
	err = runtime.SetupShutdownByDisconnect(context, cmdPort, "exec.cmd", ch)
	utils.AssertError(err)

	log.Println("Started...")
	for {
		ip, err := cmdPort.RecvMultipart(0)
		if err != nil {
			log.Println("Error receiving message:", err.Error())
			continue
		}
		if !runtime.IsValidIP(ip) {
			continue
		}
		out, err := executeCommand(string(ip[1]))
		if err != nil {
			log.Println(err.Error())
			if errPort != nil {
				errPort.SendMultipart(runtime.NewPacket([]byte(err.Error())), 0)
			}
			continue
		}
		out = bytes.Replace(out, []byte("\n"), []byte(""), -1)
		log.Println(string(out))
		if outPort != nil {
			outPort.SendMultipart(runtime.NewPacket(out), 0)
		}
	}
}
