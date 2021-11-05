package subsystem

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sergey-shpilevskiy/wails/v2/pkg/runtime"
	"strings"
	"sync"

	"github.com/sergey-shpilevskiy/wails/v2/internal/binding"
	"github.com/sergey-shpilevskiy/wails/v2/internal/logger"
	"github.com/sergey-shpilevskiy/wails/v2/internal/messagedispatcher/message"
	"github.com/sergey-shpilevskiy/wails/v2/internal/servicebus"
)

// Call is the Call subsystem. It manages all service bus messages
// starting with "call".
type Call struct {
	callChannel <-chan *servicebus.Message

	// quit flag
	shouldQuit bool

	// bindings DB
	DB *binding.DB

	// ServiceBus
	bus *servicebus.ServiceBus

	// logger
	logger logger.CustomLogger

	// context
	ctx context.Context

	// parent waitgroup
	wg *sync.WaitGroup
}

// NewCall creates a new call subsystem
func NewCall(ctx context.Context, bus *servicebus.ServiceBus, logger *logger.Logger, DB *binding.DB) (*Call, error) {

	// Subscribe to event messages
	callChannel, err := bus.Subscribe("call:invoke")
	if err != nil {
		return nil, err
	}

	result := &Call{
		callChannel: callChannel,
		logger:      logger.CustomLogger("Call Subsystem"),
		DB:          DB,
		bus:         bus,
		ctx:         ctx,
		wg:          ctx.Value("waitgroup").(*sync.WaitGroup),
	}

	return result, nil
}

// Start the subsystem
func (c *Call) Start() error {

	c.wg.Add(1)

	// Spin off a go routine
	go func() {
		defer c.logger.Trace("Shutdown")
		for {
			select {
			case <-c.ctx.Done():
				c.wg.Done()
				return
			case callMessage := <-c.callChannel:
				c.processCall(callMessage)
			}
		}

	}()

	return nil
}

func (c *Call) processCall(callMessage *servicebus.Message) {

	c.logger.Trace("Got message: %+v", callMessage)

	// Extract payload
	payload := callMessage.Data().(*message.CallMessage)

	// Lookup method
	registeredMethod := c.DB.GetMethod(payload.Name)

	// Check if it's a system call
	if strings.HasPrefix(payload.Name, ".wails.") {
		c.processSystemCall(payload, callMessage.Target())
		return
	}

	// Check we have it
	if registeredMethod == nil {
		c.sendError(fmt.Errorf("Method not registered"), payload, callMessage.Target())
		return
	}
	c.logger.Trace("Got registered method: %+v", registeredMethod)

	args, err := registeredMethod.ParseArgs(payload.Args)
	if err != nil {
		c.sendError(fmt.Errorf("Error parsing arguments: %s", err.Error()), payload, callMessage.Target())
	}

	result, err := registeredMethod.Call(args)
	if err != nil {
		c.sendError(err, payload, callMessage.Target())
		return
	}
	c.logger.Trace("registeredMethod.Call: %+v, %+v", result, err)
	// process result
	c.sendResult(result, payload, callMessage.Target())

}

func (c *Call) processSystemCall(payload *message.CallMessage, clientID string) {
	c.logger.Trace("Got internal System call: %+v", payload)
	callName := strings.TrimPrefix(payload.Name, ".wails.")
	switch callName {
	case "Dialog.Open":
		var dialogOptions runtime.OpenDialogOptions
		err := json.Unmarshal(payload.Args[0], &dialogOptions)
		if err != nil {
			c.logger.Error("Error decoding: %s", err)
		}
		result, err := runtime.OpenFileDialog(c.ctx, dialogOptions)
		if err != nil {
			c.logger.Error("Error: %s", err)
		}
		c.sendResult(result, payload, clientID)
	case "Dialog.Save":
		var dialogOptions runtime.SaveDialogOptions
		err := json.Unmarshal(payload.Args[0], &dialogOptions)
		if err != nil {
			c.logger.Error("Error decoding: %s", err)
		}
		result, err := runtime.SaveFileDialog(c.ctx, dialogOptions)
		if err != nil {
			c.logger.Error("Error: %s", err)
		}
		c.sendResult(result, payload, clientID)
	case "Dialog.Message":
		var dialogOptions runtime.MessageDialogOptions
		err := json.Unmarshal(payload.Args[0], &dialogOptions)
		if err != nil {
			c.logger.Error("Error decoding: %s", err)
		}
		result, err := runtime.MessageDialog(c.ctx, dialogOptions)
		if err != nil {
			c.logger.Error("Error: %s", err)
		}
		c.sendResult(result, payload, clientID)
	default:
		c.logger.Error("Unknown system call: %+v", callName)
	}
}

func (c *Call) sendResult(result interface{}, payload *message.CallMessage, clientID string) {
	c.logger.Trace("Sending success result with CallbackID '%s' : %+v\n", payload.CallbackID, result)
	incomingMessage := &CallbackMessage{
		Result:     result,
		CallbackID: payload.CallbackID,
	}
	messageData, err := json.Marshal(incomingMessage)
	c.logger.Trace("json incomingMessage data: %+v\n", string(messageData))
	if err != nil {
		// what now?
		c.logger.Fatal(err.Error())
	}
	c.bus.PublishForTarget("call:result", string(messageData), clientID)
}

func (c *Call) sendError(err error, payload *message.CallMessage, clientID string) {
	c.logger.Trace("Sending error result with CallbackID '%s' : %+v\n", payload.CallbackID, err.Error())
	incomingMessage := &CallbackMessage{
		Err:        err.Error(),
		CallbackID: payload.CallbackID,
	}

	messageData, err := json.Marshal(incomingMessage)
	c.logger.Trace("json incomingMessage data: %+v\n", string(messageData))
	if err != nil {
		// what now?
		c.logger.Fatal(err.Error())
	}
	c.bus.PublishForTarget("call:result", string(messageData), clientID)
}

// CallbackMessage defines a message that contains the result of a call
type CallbackMessage struct {
	Result     interface{} `json:"result"`
	Err        string      `json:"error"`
	CallbackID string      `json:"callbackid"`
}
