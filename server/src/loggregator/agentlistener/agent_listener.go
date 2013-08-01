package agentlistener

import (
	"github.com/cloudfoundry/gosteno"
	"instrumentor"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

type agentListener struct {
	*gosteno.Logger
	host                 string
	receivedMessageCount *uint64
	receivedByteCount    *uint64
	dataChannel          chan []byte
}

func NewAgentListener(host string, givenLogger *gosteno.Logger) *agentListener {
	return &agentListener{givenLogger, host, new(uint64), new(uint64), make(chan []byte, 1024)}
}

func (agentListener *agentListener) Start() chan []byte {
	connection, err := net.ListenPacket("udp", agentListener.host)
	agentListener.Infof("Listening on port %s", agentListener.host)
	if err != nil {
		agentListener.Fatalf("Failed to listen on port. %s", err)
		panic(err)
	}

	agentInstrumentor := instrumentor.NewInstrumentor(5*time.Second, gosteno.LOG_DEBUG, agentListener.Logger)
	stopChan := agentInstrumentor.Instrument(agentListener)

	go func() {
		readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size

		for {
			readCount, senderAddr, err := connection.ReadFrom(readBuffer)
			if err != nil {
				agentListener.Debugf("Error while reading. %s", err)
			}
			agentListener.Debugf("Read %d bytes from address %s", readCount, senderAddr)

			readData := make([]byte, readCount) //pass on buffer in size only of read data
			copy(readData, readBuffer[:readCount])

			atomic.AddUint64(agentListener.receivedMessageCount, 1)
			atomic.AddUint64(agentListener.receivedByteCount, uint64(readCount))
			agentListener.dataChannel <- readData
		}

		agentInstrumentor.StopInstrumentation(stopChan)
	}()
	return agentListener.dataChannel
}

func (agentListener *agentListener) DumpData() []instrumentor.PropVal {
	return []instrumentor.PropVal{
		instrumentor.PropVal{"CurrentBufferCount", strconv.Itoa(len(agentListener.dataChannel))},
		instrumentor.PropVal{"ReceivedMessageCount", strconv.FormatUint(atomic.LoadUint64(agentListener.receivedMessageCount), 10)},
		instrumentor.PropVal{"ReceivedByteCount", strconv.FormatUint(atomic.LoadUint64(agentListener.receivedByteCount), 10)},
	}
}