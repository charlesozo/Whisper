package main

import (
	"context"
	"fmt"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"log"
	"os"
	"sync"
)

var senderJIDChan = make(chan types.JID, 10)
var usernameChan = make(chan string, 10)
var messageIDChan = make(chan []types.MessageID, 10)
var chatJIDChan = make(chan types.JID, 10)
var senderNumberChan = make(chan string, 10)
var messageChan = make(chan *waProto.Message, 10)
var usersLock sync.Mutex

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.Chat.Server == "s.whatsapp.net" {
			// create a separate goroutine to process incoming messages from users
			
			go func() {
				usersLock.Lock()
				defer usersLock.Unlock()
				senderJIDChan <- v.Info.Sender
				usernameChan <- v.Info.PushName
				messageIDChan <- []types.MessageID{v.Info.ID}
				chatJIDChan <- v.Info.Chat
				senderNumberChan <- v.Info.Sender.User
				messageChan <- v.Message
				fmt.Println("GetConversation : ", v.Message.GetConversation())
				fmt.Println("Sender : ", v.Info.Sender)
				fmt.Println("Sender Number : ", v.Info.Sender.User)
				fmt.Println("IsGroup : ", v.Info.IsGroup)
				fmt.Println("MessageSource : ", v.Info.MessageSource)
				fmt.Println("ID : ", v.Info.ID)
				fmt.Println("PushName : ", v.Info.PushName)
				fmt.Println("BroadcastListOwner : ", v.Info.BroadcastListOwner)
				fmt.Println("Category : ", v.Info.Category)
				fmt.Println("Chat : ", v.Info.Chat)
				fmt.Println("DeviceSentMeta : ", v.Info.DeviceSentMeta)
				fmt.Println("IsFromMe : ", v.Info.IsFromMe)
				fmt.Println("MediaType : ", v.Info.MediaType)
				fmt.Println("Multicast : ", v.Info.Multicast)
				fmt.Println("Info.Chat.Server : ", v.Info.Chat.Server)
			}()
         
		}
	}
}

func (cfg *waConfig) handleIncomingMessages(client *whatsmeow.Client, ctx context.Context) {
	// create for loop to infinitely handle messages

	for {

		// Use a select statement to wait for either the context to be canceled
		// or the message to be processed
		select {
			//wait for when a message is sent over the channel.
		case senderJID := <-senderJIDChan:
			// assign the channels to their individual variable
			senderNumber := <-senderNumberChan
			username := <-usernameChan
			chatJID := <-chatJIDChan
			messageID := <-messageIDChan
			// mark messages can be done in a separate goroutine in order to avoid blocking
			// Mark message as read
			go MarkMessageRead(client, chatJID, senderJID, messageID)
			// handle User Messages concurrently
			 go cfg.HandleUsers(ctx, client, senderJID, username, chatJID, senderNumber, messageID, messageChan)
		case <-ctx.Done():
			// Context canceled, stop the user processing
			fmt.Println("context is cancelled from parent")
			continue
			// User processed, continue with the next message
		}
	}
}

func (cfg *waConfig) waConnect(ctx context.Context) (*whatsmeow.Client, error) {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	container, err := sqlstore.New("postgres", cfg.DBURL, dbLog)
	if err != nil {
		log.Fatalf("Unable to create a database store %v", err)
	}
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		log.Fatalf("Unable to create a device store %v", err)
	}

	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)
	go cfg.handleIncomingMessages(client, ctx)
	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(ctx)
		err = client.Connect()
		if err != nil {
			return nil, err
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else if evt.Event == "authenticated" {
				fmt.Println("User is logged in!")
				os.Exit(0) // Exit the program with a status code of 0
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		err := client.Connect()
		if err != nil {
			return nil, err
		}

	}
	return client, nil

}
