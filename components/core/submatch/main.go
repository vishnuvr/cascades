package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/cascades-fbp/cascades/components/utils"
	"github.com/cascades-fbp/cascades/runtime"
	zmq "github.com/pebbe/zmq4"
)

var (
	// Flags
	patternEndpoint = flag.String("port.pattern", "", "Component's options port endpoint")
	inputEndpoint   = flag.String("port.in", "", "Component's input port endpoint")
	mapEndpoint     = flag.String("port.map", "", "Component's output port endpoint")
	jsonFlag        = flag.Bool("json", false, "Print component documentation in JSON")
	debug           = flag.Bool("debug", false, "Enable debug mode")

	// Internal
	patternPort, inPort, mapPort *zmq.Socket
	err                          error
)

type mainRegexp struct {
	*regexp.Regexp
}

func (r *mainRegexp) FindStringSubmatchMap(s string) map[string]string {
	captures := make(map[string]string)
	match := r.FindStringSubmatch(s)
	if match == nil {
		return captures
	}
	for i, name := range r.SubexpNames() {
		if i == 0 {
			continue
		}
		captures[name] = match[i]

	}
	return captures
}

func validateArgs() {
	if *patternEndpoint == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *inputEndpoint == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *mapEndpoint == "" {
		flag.Usage()
		os.Exit(1)
	}
}

func openPorts() {
	patternPort, err = utils.CreateInputPort(*patternEndpoint)
	utils.AssertError(err)

	inPort, err = utils.CreateInputPort(*inputEndpoint)
	utils.AssertError(err)

	mapPort, err = utils.CreateOutputPort(*mapEndpoint)
	utils.AssertError(err)
}

func closePorts() {
	patternPort.Close()
	inPort.Close()
	mapPort.Close()
	zmq.Term()
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
	err = runtime.SetupShutdownByDisconnect(inPort, "submatch.in", ch)
	utils.AssertError(err)

	log.Println("Waiting for pattern...")
	var (
		pattern mainRegexp
		ip      [][]byte
	)
	for {
		ip, err = patternPort.RecvMessageBytes(0)
		if err != nil {
			log.Println("Error receiving message:", err.Error())
			continue
		}
		if !runtime.IsValidIP(ip) {
			log.Println("Invalid IP:", ip)
			continue
		}
		log.Println("Using pattern:", string(ip[1]))
		pattern = mainRegexp{regexp.MustCompile(string(ip[1]))}
		break
	}

	log.Println("Started...")
	for {
		ip, err = inPort.RecvMessageBytes(0)
		if err != nil {
			log.Println("Error receiving message:", err.Error())
			continue
		}
		if !runtime.IsValidIP(ip) {
			log.Println("Invalid IP:", ip)
			continue
		}

		matches := pattern.FindStringSubmatchMap(string(ip[1]))
		log.Printf("Matches: %#v\n", matches)

		data, err := json.Marshal(matches)
		if err != nil {
			log.Println(err.Error())
		}

		mapPort.SendMessage(runtime.NewPacket(data))
	}
}
