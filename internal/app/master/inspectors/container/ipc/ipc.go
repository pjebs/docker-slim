package ipc

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/req"
	"github.com/go-mangos/mangos/protocol/sub"
	//"github.com/go-mangos/mangos/transport/ipc"
	"github.com/go-mangos/mangos/transport/tcp"

	"github.com/docker-slim/docker-slim/pkg/ipc/channel"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
)

// InitContainerChannels initializes the communication channels with the target container
func InitContainerChannels(dockerHostIP, cmdChannelPort, evtChannelPort string) error {
	cmdChannelAddr = fmt.Sprintf("tcp://%v:%v", dockerHostIP, cmdChannelPort)
	evtChannelAddr = fmt.Sprintf("tcp://%v:%v", dockerHostIP, evtChannelPort)
	log.Debugf("cmdChannelAddr=%v evtChannelAddr=%v", cmdChannelAddr, evtChannelAddr)

	//evtChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-sensor.events.ipc", localVolumePath)
	//cmdChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-sensor.cmds.ipc", localVolumePath)

	var err error
	evtChannel, err = newEvtChannel(evtChannelAddr)
	if err != nil {
		return err
	}
	cmdChannel, err = newCmdClient(cmdChannelAddr)
	if err != nil {
		return err
	}

	return nil
}

// SendContainerCmd sends the given command to the target container
func SendContainerCmd(cmd command.Message) (string, error) {
	return sendCmd(cmdChannel, cmd)
}

// GetContainerEvt returns the current event generated by the target container
func GetContainerEvt() (event.Name, error) {
	return getEvt(evtChannel)
}

// ShutdownContainerChannels destroys the communication channels with the target container
func ShutdownContainerChannels() {
	shutdownEvtChannel()
	shutdownCmdChannel()
}

//var cmdChannelAddr = "ipc:///tmp/docker-slim-sensor.cmds.ipc"
var cmdChannelAddr = fmt.Sprintf("tcp://127.0.0.1:%d", channel.CmdPort)

var cmdChannel mangos.Socket

func newCmdClient(addr string) (mangos.Socket, error) {
	socket, err := req.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionSendDeadline, time.Second*3); err != nil {
		socket.Close()
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionRecvDeadline, time.Second*3); err != nil {
		socket.Close()
		return nil, err
	}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil, err
	}

	return socket, nil
}

func shutdownCmdChannel() {
	if cmdChannel != nil {
		cmdChannel.Close()
		cmdChannel = nil
	}
}

func sendCmd(channel mangos.Socket, cmd command.Message) (string, error) {
	sendTimeouts := 0
	recvTimeouts := 0

	log.Debugf("sendCmd(%s)", cmd)
	for {
		sendData, err := command.Encode(cmd)
		if err != nil {
			log.Info("sendCmd(): malformed cmd - ", err)
			return "", err
		}

		if err := channel.Send(sendData); err != nil {
			switch err {
			case mangos.ErrSendTimeout:
				log.Info("sendCmd(): send timeout...")
				sendTimeouts++
				if sendTimeouts > 3 {
					return "", err
				}
			default:
				return "", err
			}
		}

		response, err := channel.Recv()
		if err != nil {
			switch err {
			case mangos.ErrRecvTimeout:
				log.Info("sendCmd(): receive timeout...")
				recvTimeouts++
				if recvTimeouts > 3 {
					return "", err
				}
			default:
				return "", err
			}
		}

		return string(response), nil
	}
}

var evtChannelAddr = fmt.Sprintf("tcp://127.0.0.1:%d", channel.EvtPort)

//var evtChannelAddr = "ipc:///tmp/docker-slim-sensor.events.ipc"
var evtChannel mangos.Socket

func newEvtChannel(addr string) (mangos.Socket, error) {
	socket, err := sub.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionRecvDeadline, time.Second*120); err != nil {
		socket.Close()
		return nil, err
	}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil, err
	}

	err = socket.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return nil, err
	}

	return socket, nil
}

func shutdownEvtChannel() {
	if evtChannel != nil {
		evtChannel.Close()
		evtChannel = nil
	}
}

func getEvt(channel mangos.Socket) (event.Name, error) {
	log.Debug("getEvt()")
	evt, err := channel.Recv()
	log.Debug("getEvt(): channel.Recv() - done")
	if err != nil {
		return "", err
	}

	return event.Name(evt), nil
}
