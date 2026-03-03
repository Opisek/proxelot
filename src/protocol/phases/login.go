package phases

import (
	"errors"
	"fmt"
	"mginx/config"
	"mginx/connections/upstream"
	"mginx/models"
	"mginx/protocol/dialog"
	util "mginx/protocol/internal"
	"mginx/protocol/parsing"
	"mginx/protocol/payloads"
	"mginx/protocol/serializing"
)

func HandleLoginPhase(client *models.DownstreamClient, packet payloads.GenericPacket, conf *config.Configuration) error {
	switch packet.Id {
	case 0x00:
		err := handleClientLoginStart(client, packet)
		if err != nil {
			return errors.Join(errors.New("could not parse login start packet"), err)
		}
	case 0x03:
		err := handleClientLoginAcknowledged(client, packet)
		if err != nil {
			return errors.Join(errors.New("could not parse login acknowledged packet"), err)
		}
	default:
		return fmt.Errorf("invalid packet id: %v", packet.Id)
	}
	return nil
}

func handleClientLoginStart(client *models.DownstreamClient, packet payloads.GenericPacket) error {
	fmt.Println("login start")
	payload, err := parsing.ParseLoginStart(packet.Payload)

	if err != nil {
		return err
	}

	client.Username = payload.Name
	client.Uuid = payload.Uuid

	// If we are working with redirects, acknowledge to move into the configuration phase
	if client.Upstream.Redirect {
	}

	// If, we do not support redirects, we want to create a proxy channel if the server is up
	if !client.Upstream.Redirect {
		reconnectTriggerChannel := make(chan bool)

		alreadyUp := client.Upstream.Connect(client, func() {
			// If we are able to connect immediately, create a proxy channel
			actualAddress, err := upstream.ProxyConnection(client)
			if err != nil {
				panic(errors.Join(errors.New("could not proxy connection"), err))
				// TODO: disconnect
			}

			client.UpstreamConnection.Write(serializing.SerializeHandshake(payloads.Handshake{
				Version: client.Version,
				Address: actualAddress,
				Port:    client.Port,
				Intent:  0x02,
			}))

			client.UpstreamConnection.Write(serializing.SerializeLoginStart(payload))

			client.EnableProxying()
		}, func() {
			// If we first need to go into a waiting queue, we will have to reconnect once the server is up
			<-reconnectTriggerChannel
			client.Connection.Write(serializing.SerializeTransfer(payloads.Transfer{
				Host: client.Address,
				Port: client.Port,
			}))
			client.StartTransfer()
		})

		if alreadyUp {
			return nil
		}
		client.RegisterLoginPhaseChannel(reconnectTriggerChannel)
	}

	// If we use redirects or the server is not yet up, go into the login phase
	client.Connection.Write(serializing.SerializeLoginSuccess(payloads.LoginSuccess{
		Name: client.Username,
		Uuid: client.Uuid,
	}))

	return nil
}

func handleClientLoginAcknowledged(client *models.DownstreamClient, packet payloads.GenericPacket) error {
	fmt.Println("login ack")
	_, err := parsing.ParseLoginAcknowledged(packet.Payload)

	if err != nil {
		return err
	}

	client.GamePhase = 0x04

	client.LoginFinished()
	if client.Upstream.Redirect {
		alreadyConnected := client.Upstream.Connect(client, func() {
			client.Connection.Write(serializing.SerializeTransfer(payloads.Transfer{
				Host: client.Upstream.To.Hostname,
				Port: client.Upstream.To.Port,
			}))
			client.StartTransfer()
		}, func() {})

		if alreadyConnected {
			return nil
		}
	}

	client.ExpectedKeepalive, err = util.GetRandomLong()
	if err != nil {
		return errors.Join(errors.New("could not generate a random keepalive ID"), err)
	}

	client.Connection.Write(serializing.SerializeKeepAlive(payloads.KeepAlive{
		Id: client.ExpectedKeepalive,
	}))

	client.Connection.Write(serializing.SerializeShowDialog(payloads.ShowDialog{Dialog: dialog.Dialog{
		Title:       "Server Startup",
		Description: "The server is starting up.\n\nYou will be connected shortly.\n\nPlease wait...",
	}}))

	return nil
}
