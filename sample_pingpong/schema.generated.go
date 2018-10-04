package main

import (
    "bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

    "gitlab.com/exashape/rpckit2"
)

type ServerMethod uint64

type ServerClient struct {
    c *RPCConnection
}

func NewServerClient(c *RPCConnection) *ServerClient {
	return &ServerClient{c: c}
}

type rpcMessage interface {
	RPCEncode(m *message) error
	RPCID() uint64
}

type rpcCallHandler interface {
	RPCCall(methodID uint64, m *message) rpcMessage
}

// *************************

type message struct {
	buf       []byte
	len       int
	pos       int // for reading
	LastError error
}

func newMessage(capacity int) *message {
	m := &message{
		buf: make([]byte, 4+capacity),
		len: 4, // make room for length prefix
		pos: 4, // make room for length prefix
	}
	return m
}

func messageFromBytes(msg []byte) (*message, int) {
	if len(msg) > 4 {
		length := int(binary.BigEndian.Uint32(msg))
		if len(msg) >= length {
			m := &message{
				buf: msg,
				len: len(msg),
				pos: 4,
			}
			return m, length
		}
	}
	return nil, 0
}

func (m *message) Bytes() []byte {
	b := m.buf[:m.len]
	binary.BigEndian.PutUint32(b, uint32(m.len))
	return b
}

func (m *message) WriteVarint(v uint64) {
	m.grow(9)
	//i := 0
	for v >= 0x80 {
		m.buf[m.len] = byte(v) | 0x80
		m.len++
		v >>= 7
		//i++
	}
	m.buf[m.len] = byte(v)
	m.len++
}

func (m *message) WriteString(v string) {
	stringLength := len(v)
	m.WriteVarint(uint64(stringLength))

	m.grow(stringLength)
	copy(m.buf[m.len:], v)
	m.len += stringLength
}

func (m *message) grow(needed int) {
	if m.len+needed > len(m.buf) {
		// Not enough space anywhere, we need to allocate.
		buf := makeSlice(2*cap(m.buf) + needed)
		copy(buf, m.buf[0:m.len])
		m.buf = buf
	}
}

func (m *message) ReadString() (string, error) {
	v, err := m.ReadVarint()
	if err != nil {
		m.LastError = err
		return "", err
	}
	length := int(v)

	if m.pos+length > m.len {
		m.LastError = io.EOF
		return "", io.EOF
	}

	str := m.buf[m.pos : m.pos+length]
	m.pos += length

	return string(str), nil
}

var ErrOverflow = errors.New("Overflow in varint")

func (m *message) ReadVarint() (uint64, error) {
	var x uint64
	var s uint
	for i := 0; ; i++ {
		if m.pos+1 > m.len {
			m.LastError = io.EOF
			return 0, io.EOF
		}
		b := m.buf[m.pos]
		m.pos++
		if b < 0x80 {
			if i > 9 || (i == 9 && b > 1) {
				m.LastError = ErrOverflow
				return 0, ErrOverflow
			}
			return x | uint64(b)<<s, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}

func (m *message) ReadBool() (bool, error) {
	v, err := m.ReadVarint()
	if err != nil {
		return false, err
	}
	return v == 1, nil
}

type wireType uint64

const (
	wireTypeVarint          wireType = 0
	wireType64bit           wireType = 1
	wireTypeLengthDelimited wireType = 2
	wireType32bit           wireType = 5
)

func (w wireType) String() string {
	switch w {
	case wireTypeVarint:
		return "wireTypeVarint"
	case wireType64bit:
		return "wireType64bit"
	case wireTypeLengthDelimited:
		return "wireTypeLengthDelimited"
	case wireType32bit:
		return "wireType32bit"
	default:
		panic("unknown wiretype")
	}
}

func wireTypeForPropertyType(t rpckit2.PropertyType) wireType {
	switch t {
	case rpckit2.String:
		return wireTypeLengthDelimited
	case rpckit2.Bool:
		return wireTypeVarint
	case rpckit2.Int64:
		return wireType64bit
	case rpckit2.Int:
		panic("int not implemented")
	default:
		panic("unknown property type")
	}
}

func (m *message) ReadPBSkip(tag uint64) error {
	var err error
	switch wireType(tag & ((1 << 3) - 1)) {
	case wireTypeVarint:
		_, err = m.ReadVarint()
	case wireType64bit:
		// TODO
		break
	case wireTypeLengthDelimited:
		// TODO
		break
	case wireType32bit:
		// TODO
		break
	}
	return err
}

func (m *message) WritePBTag(fieldNumber uint64, wireType wireType) {
	m.WriteVarint((fieldNumber << 3) | uint64(wireType))
}

func (m *message) WritePBString(fieldNumber uint64, value string) {
	if value != "" {
		m.WritePBTag(fieldNumber, wireTypeLengthDelimited)
		m.WriteString(value)
	}
}

func (m *message) WritePBBool(fieldNumber uint64, value bool) {
	if value != false {
		m.WritePBTag(fieldNumber, wireTypeVarint)
		m.WriteVarint(1)
	}
}

// ************************
type RPCErrorID uint64

const (
	GenericError  RPCErrorID = 0
	TimeoutError  RPCErrorID = 1
	ProtocolError RPCErrorID = 2
)

type RPCError interface {
	error
	ID() RPCErrorID
}

type rpcError struct {
	id    RPCErrorID
	error string
}

func (e *rpcError) ID() RPCErrorID {
	return e.id
}

func (e *rpcError) Error() string {
	return e.error
}

// ************************

type DisconnectedHandler func(c *RPCConnection, err RPCError)

type RPCConnection struct {
	sync.RWMutex
	connected    bool
	conn         net.Conn
	onDisconnect DisconnectedHandler
	handlers     map[uint64]rpcCallHandler
	waitingCalls map[uint64]chan *message
}

var rpcPreamble = []byte{82, 80, 67, 75, 73, 84, 0, 0, 0, 1} //R,P,C,K,I,T .. version 1
func NewRPCConnection(conn net.Conn) (*RPCConnection, RPCError) {
	c := &RPCConnection{
		connected:    true,
		conn:         conn,
		handlers:     make(map[uint64]rpcCallHandler),
		waitingCalls: make(map[uint64]chan *message),
	}

	// send the preable
	n, err := c.conn.Write(rpcPreamble)
	if err != nil {
		return nil, &rpcError{id: GenericError, error: fmt.Sprintf("Could not write preamble. Details: %v", err)}
	}
	if n != len(rpcPreamble) {
		return nil, &rpcError{id: ProtocolError, error: "Could not send entire protocol preamble."}
	}

	go c.readLoop()
	return c, nil
}

func (c *RPCConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *RPCConnection) readLoop() {
	receivedPreamble := false
	buf := make([]byte, 0, 512)
	off := 0
	minRead := 256

	// kill bad connections
	defer func() {
		if bad := recover(); bad != nil {
			c.end(&rpcError{id: GenericError, error: fmt.Sprintf("unhandled error in readLoop(): %v", bad)})
		}
	}()

	for {
		// read from the connection
		if off >= len(buf) {
			buf = buf[:0]
			off = 0
		} // }

		if free := cap(buf) - len(buf); free < minRead {
			//fmt.Println("grow")
			// not enough space at end
			newBuf := buf
			if off+free < minRead {
				// not enough space using beginning of buffer;
				// double buffer capacity
				newBuf = makeSlice(2*cap(buf) + minRead)
			}
			copy(newBuf, buf[off:])
			buf = newBuf[:len(buf)-off]
			off = 0
		}
		m, err := c.conn.Read(buf[len(buf):cap(buf)])
		buf = buf[0 : len(buf)+m]
		if err == io.EOF {
			c.end(nil)
			return
		}
		if err != nil {
			c.end(&rpcError{id: GenericError, error: fmt.Sprintf("error reading from connection in readLoop(): %v", err)})
			return
		}

		for {
			// check for premable
			if !receivedPreamble {
				if len(buf[off:]) >= len(rpcPreamble) {
					if bytes.Equal(rpcPreamble, buf[off:off+len(rpcPreamble)]) {
						off += len(rpcPreamble)
						receivedPreamble = true
					} else {
						c.end(&rpcError{id: ProtocolError, error: "The first bytes received from the other end was not the expected rpckit preamble"})
						return
					}
				} else { // }

					break
				}
			}

			// parse messages
			msg, length := messageFromBytes(buf[off:])
			if msg != nil {
				off += length
				c.gotMessage(msg)
			} else {
				break
			}
		}
	}
}

func (c *RPCConnection) gotMessage(msg *message) {
	t, err := msg.ReadVarint() // what kind of message is it
	if err != nil {
		c.end(&rpcError{id: ProtocolError, error: "Could not read message type as the first varint in the incomming message."})
		return
	}

	switch t {
	case 1: // method call
		callID, err := msg.ReadVarint() // what is the id of this call?
		if err != nil {
			c.end(&rpcError{id: ProtocolError, error: "Could not read callId from message call."})
			return
		}
		protocolID, err := msg.ReadVarint() // what is the protocol being called?
		if err != nil {
			c.end(&rpcError{id: ProtocolError, error: "Could not read protocolId from message call. Closed connection."})
			return
		}

		handler, found := c.handlers[protocolID]
		if !found {
			c.end(&rpcError{id: ProtocolError, error: fmt.Sprintf("Unknown protocol: %v", protocolID)})
			return
		}

		methodID, err := msg.ReadVarint() // what is the method being called?
		if err != nil {
			c.end(&rpcError{id: ProtocolError, error: "Could not read methodID from message call. Closed connection."})
			return
		}

		result := handler.RPCCall(methodID, msg)
		if result != nil {
			reply := newMessage(1024)
			reply.WriteVarint(2)      // method return
			reply.WriteVarint(callID) // callid
			reply.WriteVarint(result.RPCID())
			result.RPCEncode(reply)
			c.send(reply)
		} // }

		break
	case 2: // method return
		callID, err := msg.ReadVarint() // what is the id of this call?
		if err != nil {
			c.end(&rpcError{id: ProtocolError, error: "Could not read callId from message call. Closed connection."})
			return
		}

		c.RLock()
		ch, found := c.waitingCalls[callID]
		c.RUnlock()
		if !found {
			c.end(&rpcError{id: ProtocolError, error: fmt.Sprintf("Could not find a waiting call for the call id: %v", callID)})
			return
		}

		ch <- msg
		break
	}
}

func (c *RPCConnection) logError(err RPCError) {
	fmt.Println("ERROR", err.Error())
}

func (c *RPCConnection) call(waitForReply bool, protocolID uint64, methodID uint64, callArgs rpcMessage) (uint64, *message, RPCError) {
	// create the call message
	callID := uint64(1234) // TODO:  proper call id.
	m := newMessage(1024)
	m.WriteVarint(1)          // it's a method call
	m.WriteVarint(callID)     // method callid)
	m.WriteVarint(protocolID) // protocol id
	m.WriteVarint(methodID)   // method id
	err := callArgs.RPCEncode(m)
	if err != nil {
		return 0, nil, &rpcError{id: GenericError, error: fmt.Sprintf("Unable to encode call args: %v", err)}
	} // }

	if !waitForReply {
		return 0, nil, c.send(m)
	}

	// setup the call and timer
	ch := make(chan *message, 1)
	timer := time.NewTimer(5 * time.Second)
	c.Lock()
	c.waitingCalls[callID] = ch
	c.Unlock()
	defer func() {
		close(ch)
		timer.Stop()
		c.Lock()
		delete(c.waitingCalls, callID)
		c.Unlock()
	}()

	// send the call over to the otherside
	sendErr := c.send(m)
	if sendErr != nil {
		return 0, nil, sendErr
	}

	// wait for the reply
	select {
	case msg := <-ch:
		resultTypeID, err := msg.ReadVarint()
		if err != nil {
			c.Close()
			return 0, nil, &rpcError{id: GenericError, error: fmt.Sprintf("Invalid message format received. Closed connection. Details: %v", err)}
		}
		return resultTypeID, msg, nil
	case <-timer.C:
		return 0, nil, &rpcError{id: TimeoutError, error: "Timed out waiting for reply"}
	}
}

func (c *RPCConnection) send(msg *message) RPCError {
	bytes := msg.Bytes()
	n, err := c.conn.Write(bytes)
	if err != nil {
		return &rpcError{id: GenericError, error: fmt.Sprintf("Could not write message on connection: %v", err)}
	}
	if n != len(bytes) {
		c.Close()
		return &rpcError{id: ProtocolError, error: "Message was not fully sent. Closed connection."}
	}
	return nil
}

func (c *RPCConnection) Close() {
	c.end(nil)
}

func (c *RPCConnection) end(err RPCError) {
	c.Lock()
	defer c.Unlock()
	if c.connected {
		c.connected = false
		c.conn.Close()
		d := c.onDisconnect
		if d != nil {
			c.onDisconnect = nil
			d(c, err)
		}
	}
}

func (c *RPCConnection) setHandler(protocolID uint64, handler rpcCallHandler) {
	c.handlers[protocolID] = handler
}

func makeSlice(n int) []byte {
	// If the make fails, give a known error.
	defer func() {
		if recover() != nil {
			panic(bytes.ErrTooLarge)
		}
	}()
	return make([]byte, n)
}


const ServerMethodAuthenticate ServerMethod = 1

type serverRequest_Authenticate struct { 
    username string
    password string
}

func (s *serverRequest_Authenticate) RPCID() uint64 {
    return 1
}

func (s *serverRequest_Authenticate) RPCEncode(m *message) error { 
    m.WritePBString(1, s.username)
    m.WritePBString(2, s.password)
    return nil
}

func (s *serverRequest_Authenticate) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag { 
        case uint64(1 << 3) | uint64(wireTypeForPropertyType(rpckit2.String)):
            s.username, err = m.ReadString()
        case uint64(2 << 3) | uint64(wireTypeForPropertyType(rpckit2.String)):
            s.password, err = m.ReadString()
        default:
            if err != io.EOF {
                err = m.ReadPBSkip(tag)
            }
        }
    }
    if err == io.EOF {
        return nil
    }
    return err
}

type serverResponse_Authenticate struct { 
    success bool
}

func (s *serverResponse_Authenticate) RPCID() uint64 {
    return 2
}

func (s *serverResponse_Authenticate) RPCEncode(m *message) error { 
    m.WritePBBool(1, s.success)
    return nil
}

func (s *serverResponse_Authenticate) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag { 
        case uint64(1 << 3) | uint64(wireTypeForPropertyType(rpckit2.Bool)):
            s.success, err = m.ReadBool()
        default:
            if err != io.EOF {
                err = m.ReadPBSkip(tag)
            }
        }
    }
    if err == io.EOF {
        return nil
    }
    return err
}

func (c *ServerClient) Authenticate( 
        username string,
        password string,
    ) ( 
        success bool,
        err RPCError,
    ) {

    resultTypeID, msg, err := c.c.call(true, 1, uint64(ServerMethodAuthenticate), &serverRequest_Authenticate{ 
        username: username,
        password: password,
    })

    if resultTypeID == 2 {
        var r serverResponse_Authenticate
        decodeError := r.RPCDecode(msg)
        if decodeError != nil {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", decodeError)}
            return
        }
        
        success = r.success
        return
    }

    fmt.Println("SOME ERROR RETURNED", resultTypeID)
    return
}


const ServerMethodPingWithReply ServerMethod = 2

type serverRequest_PingWithReply struct { 
    name string
}

func (s *serverRequest_PingWithReply) RPCID() uint64 {
    return 1
}

func (s *serverRequest_PingWithReply) RPCEncode(m *message) error { 
    m.WritePBString(1, s.name)
    return nil
}

func (s *serverRequest_PingWithReply) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag { 
        case uint64(1 << 3) | uint64(wireTypeForPropertyType(rpckit2.String)):
            s.name, err = m.ReadString()
        default:
            if err != io.EOF {
                err = m.ReadPBSkip(tag)
            }
        }
    }
    if err == io.EOF {
        return nil
    }
    return err
}

type serverResponse_PingWithReply struct { 
    greeting string
}

func (s *serverResponse_PingWithReply) RPCID() uint64 {
    return 2
}

func (s *serverResponse_PingWithReply) RPCEncode(m *message) error { 
    m.WritePBString(1, s.greeting)
    return nil
}

func (s *serverResponse_PingWithReply) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag { 
        case uint64(1 << 3) | uint64(wireTypeForPropertyType(rpckit2.String)):
            s.greeting, err = m.ReadString()
        default:
            if err != io.EOF {
                err = m.ReadPBSkip(tag)
            }
        }
    }
    if err == io.EOF {
        return nil
    }
    return err
}

func (c *ServerClient) PingWithReply( 
        name string,
    ) ( 
        greeting string,
        err RPCError,
    ) {

    resultTypeID, msg, err := c.c.call(true, 1, uint64(ServerMethodPingWithReply), &serverRequest_PingWithReply{ 
        name: name,
    })

    if resultTypeID == 2 {
        var r serverResponse_PingWithReply
        decodeError := r.RPCDecode(msg)
        if decodeError != nil {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", decodeError)}
            return
        }
        
        greeting = r.greeting
        return
    }

    fmt.Println("SOME ERROR RETURNED", resultTypeID)
    return
}


type ServerMethods interface { 
    Authenticate( 
        username string,
        password string,
    ) ( 
        success bool,
        err error,
    )
    PingWithReply( 
        name string,
    ) ( 
        greeting string,
        err error,
    )
}

func RegisterRPCCallHandler(c *RPCConnection, s ServerMethods) {
    c.setHandler(1, &serverMethodsCallHandler{methods: s})
}

type serverMethodsCallHandler struct {
    methods ServerMethods
}

func (s *serverMethodsCallHandler) RPCCall(methodID uint64, m *message) rpcMessage {
    switch methodID { 
    case 1:
        args := serverRequest_Authenticate{}
        if err := args.RPCDecode(m); err != nil {
            // TODO: Wrap error
            return nil
        }
        success, err := s.methods.Authenticate( 
            args.username,
            args.password,
        )
        if err != nil {
            if rpcMsg, ok := err.(rpcMessage); ok {
                return rpcMsg
            }
            // TODO: Wrap error
            return nil
        }
        return &serverResponse_Authenticate{ 
            success: success,
        }
    case 2:
        args := serverRequest_PingWithReply{}
        if err := args.RPCDecode(m); err != nil {
            // TODO: Wrap error
            return nil
        }
        greeting, err := s.methods.PingWithReply( 
            args.name,
        )
        if err != nil {
            if rpcMsg, ok := err.(rpcMessage); ok {
                return rpcMsg
            }
            // TODO: Wrap error
            return nil
        }
        return &serverResponse_PingWithReply{ 
            greeting: greeting,
        }
    default:
        // TODO: Return error
        return nil
    }
}
