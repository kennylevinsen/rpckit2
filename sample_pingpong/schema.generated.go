package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"sync/atomic"
)

const (
	rpcRequest = 1
	rpcResponse = 2
)

const (
	privateTypeRPCError uint64 = math.MaxUint64
)

const (
	messageTypeMethodCall = 1
	messageTypeMethodReturn = 2
	messageTypePing = 3
	messageTypePong = 4
)

const messageCapacity = 1024

var (
	ErrOverflow = errors.New("overflow in varint")
	rpcPreamble = []byte{82, 80, 67, 75, 73, 84, 0, 0, 0, 1} //R,P,C,K,I,T .. version 1
)

type rpcMessage interface {
	RPCEncode(m *message) error
	RPCID() uint64
}

type rpcCallHandler interface {
	RPCCall(ctx context.Context, methodID uint64, m *message) rpcMessage
}

// *************************

type message struct {
	buf       []byte
	len       int
	pos       int // for reading
	LastError error
	embedded  bool
}

func newMessage(capacity int) *message {
	m := &message{
		buf: make([]byte, 4+capacity),
		len: 4, // make room for length prefix
		pos: 4, // make room for length prefix
	}
	return m
}

func newEmbeddedMessage(capacity int) *message {
	m := &message{
		buf: make([]byte, 4+capacity),
		embedded: true,
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

func embeddedMessageFromBytes(msg []byte) *message {
	m := &message{
		buf: msg,
		len: len(msg),
		embedded: true,
	}
	return m
}

func (m *message) Bytes() []byte {
	b := m.buf[:m.len]
	if !m.embedded {
		binary.BigEndian.PutUint32(b, uint32(m.len))
	}
	return b
}

func (m *message) WriteVarint(v uint64) {
	m.grow(9)
	for v >= 0x80 {
		m.buf[m.len] = byte(v) | 0x80
		m.len++
		v >>= 7
	}
	m.buf[m.len] = byte(v)
	m.len++
}

func (m *message) WriteBytes(v []byte) {
	stringLength := len(v)
	m.WriteVarint(uint64(stringLength))

	m.grow(stringLength)
	copy(m.buf[m.len:], v)
	m.len += stringLength
}

func (m *message) WriteString(v string) {
	stringLength := len(v)
	m.WriteVarint(uint64(stringLength))

	m.grow(stringLength)
	copy(m.buf[m.len:], v)
	m.len += stringLength
}

func (m *message) Write32Bit(v uint32) {
	m.grow(4)
	binary.LittleEndian.PutUint32(m.buf[m.len:], v)
	m.len += 4
}

func (m *message) Write64Bit(v uint64) {
	m.grow(8)
	binary.LittleEndian.PutUint64(m.buf[m.len:], v)
	m.len += 8
}

func (m *message) grow(needed int) {
	if m.len+needed > len(m.buf) {
		// Not enough space anywhere, we need to allocate.
		buf := makeSlice(2*cap(m.buf) + needed)
		copy(buf, m.buf[0:m.len])
		m.buf = buf
	}
}

func (m *message) ReadBytes() ([]byte, error) {
	v, err := m.ReadVarint()
	if err != nil {
		m.LastError = err
		return nil, err
	}
	length := int(v)

	if m.pos+length > m.len {
		m.LastError = io.EOF
		return nil, io.EOF
	}

	str := m.buf[m.pos : m.pos+length]
	m.pos += length

	return str, nil
}

func (m *message) ReadString() (string, error) {
	str, err := m.ReadBytes()
	return string(str), err
}

func (m *message) ReadEmbeddedMessage() (*message, error) {
	b, err := m.ReadBytes()
	if err != nil {
		return nil, err
	}
	return embeddedMessageFromBytes(b), nil
}

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

func (m *message) ReadInt() (int64, error) {
	x, err := m.ReadVarint()
	return int64(x), err
}

func (m *message) ReadBool() (bool, error) {
	v, err := m.ReadVarint()
	if err != nil {
		return false, err
	}
	return v == 1, nil
}

func (m *message) Read32Bit() (uint32, error) {
	if m.pos+4 > m.len {
		m.LastError = io.EOF
		return 0, io.EOF
	}

	x := binary.LittleEndian.Uint32(m.buf[m.pos:m.pos+4])
	m.pos += 4
	return x, nil
}

func (m *message) Read64Bit() (uint64, error) {
	if m.pos+8 > m.len {
		m.LastError = io.EOF
		return 0, io.EOF
	}

	x := binary.LittleEndian.Uint64(m.buf[m.pos:m.pos+8])
	m.pos += 8
	return x, nil
}

func (m *message) ReadFloat() (float32, error) {
	u, err := m.Read32Bit()
	if err != nil {
		return 0, err
	}

	return math.Float32frombits(u), nil
}

func (m *message) ReadDouble() (float64, error) {
	u, err := m.Read64Bit()
	if err != nil {
		return 0, err
	}

	return math.Float64frombits(u), nil
}

func (m *message) ReadInt64() (int64, error) {
	u, err := m.Read64Bit()
	return int64(u), err
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

func (m *message) ReadPBSkip(tag uint64) error {
	var err error
	switch wireType(tag & ((1 << 3) - 1)) {
	case wireTypeVarint:
		_, err = m.ReadVarint()
	case wireType64bit:
		_, err = m.Read64Bit()
	case wireTypeLengthDelimited:
		_, err = m.ReadString()
	case wireType32bit:
		_, err = m.Read32Bit()
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

func (m *message) WritePBBytes(fieldNumber uint64, value []byte) {
	if value != nil {
		m.WritePBTag(fieldNumber, wireTypeLengthDelimited)
		m.WriteBytes(value)
	}
}

func (m *message) WriteBool(value bool) {
	if value {
		m.WriteVarint(1)
	} else {
		m.WriteVarint(0)
	}
}

func (m *message) WritePBBool(fieldNumber uint64, value bool) {
	if value != false {
		m.WritePBTag(fieldNumber, wireTypeVarint)
		m.WriteBool(value)
	}
}

func (m *message) WriteInt64(value int64) {
	m.Write64Bit(uint64(value))
}

func (m *message) WritePBInt64(fieldNumber uint64, value int64) {
	if value != 0 {
		m.WritePBTag(fieldNumber, wireType64bit)
		m.WriteInt64(value)
	}
}

func (m *message) WriteInt(value int64) {
	m.WriteVarint(uint64(value))
}

func (m *message) WritePBInt(fieldNumber uint64, value int64) {
	if value != 0 {
		m.WritePBTag(fieldNumber, wireTypeVarint)
		m.WriteInt(value)
	}
}

func (m *message) WriteFloat(value float32) {
	m.Write32Bit(math.Float32bits(value))
}

func (m *message) WritePBFloat(fieldNumber uint64, value float32) {
	if value != 0 {
		m.WritePBTag(fieldNumber, wireType32bit)
		m.WriteFloat(value)
	}
}

func (m *message) WriteDouble(value float64) {
	m.Write64Bit(math.Float64bits(value))
}

func (m *message) WritePBDouble(fieldNumber uint64, value float64) {
	if value != 0 {
		m.WritePBTag(fieldNumber, wireType64bit)
		m.WriteDouble(value)
	}
}

func (m *message) WritePBMessage(fieldNumber uint64, em *message) {
	if em != nil {
		m.WritePBBytes(fieldNumber, em.Bytes())
	}
}

// ************************
type RPCErrorID uint64

const (
	GenericError     RPCErrorID = 0
	TimeoutError     RPCErrorID = 1
	ProtocolError    RPCErrorID = 2
	ApplicationError RPCErrorID = 3
	ConnectionError  RPCErrorID = 4
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

func (e *rpcError) RPCID() uint64 {
	return privateTypeRPCError
}

func (e *rpcError) RPCEncode(m *message) error {
	m.WritePBInt(1, int64(e.id))
	m.WritePBString(2, e.error)
	return nil
}

func (e *rpcError) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
		switch tag {
		case uint64(1 << 3) | uint64(wireTypeVarint):
			var id int64
			id, err = m.ReadInt()
			e.id = RPCErrorID(id)
		case uint64(2 << 3) | uint64(wireTypeLengthDelimited):
			e.error, err = m.ReadString()
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

func isRPCError(typeID uint64) bool {
	return typeID == privateTypeRPCError
}

// ************************

type connection struct {
	ready     chan struct{}
	connected bool
	endReason error
	conn      net.Conn
	writeLock sync.Mutex

	callIDLock   sync.RWMutex
	waitingCalls map[uint64]chan *message
	nextID       uint32
}


func (c *connection) acquireCallSlot() (uint64, RPCError) {
	c.callIDLock.Lock()
	defer c.callIDLock.Unlock()

	// Check if any slots exist
	if len(c.waitingCalls) >= math.MaxUint32 {
		return 0, &rpcError{id: GenericError, error: "no callID slots available"}
	}

	// Find a slot
	var n uint32
	taken := true
	for taken {
		n = atomic.AddUint32(&c.nextID, 1)
		_, taken = c.waitingCalls[uint64(n)]
	}

	// Take the slot
	c.waitingCalls[uint64(n)] = nil
	return uint64(n), nil
}

func (c *connection) abandonCallSlot(callID uint64) {
	c.callIDLock.Lock()
	defer c.callIDLock.Unlock()

	if callID > math.MaxUint32 {
		panic("attempted to abandon invalid callID")
	}

	v, ok := c.waitingCalls[callID]
	if !ok {
		panic("attempted to abandon free call slot")
	}
	if v != nil {
		panic("attempted to abandon used call slot")
	}
	delete(c.waitingCalls, callID)
}

func (c *connection) releaseCallSlot(callID uint64) {
	c.callIDLock.Lock()
	defer c.callIDLock.Unlock()

	if callID > math.MaxUint32 {
		panic("attempted to release invalid callID")
	}

	v, ok := c.waitingCalls[callID]
	if !ok {
		panic("attempted to release free call slot")
	}
	if v == nil {
		panic("attempted to release unused call slot")
	}
	close(v)
	delete(c.waitingCalls, callID)
}

func newConnection() *connection {
	return &connection{
		ready:        make(chan struct{}, 0),
		connected:    true,
		waitingCalls: make(map[uint64]chan *message),
	}
}

type rpcServer struct {
	ProtocolID uint64
	Handler rpcCallHandler
}

type Dialer              func() (net.Conn, error)
type ConnectedHandler    func(c *RPCConnection) error
type DisconnectedHandler func(c *RPCConnection, err RPCError)

type RPCConnection struct {
	done         chan struct{}
	handlers     map[uint64]rpcCallHandler
	dialer       Dialer
	onDisconnect DisconnectedHandler
	onConnect    ConnectedHandler

	conn *connection
}

type RPCBuilder struct {
	conn           net.Conn
	dialer         Dialer
	connectHook    ConnectedHandler
	disconnectHook DisconnectedHandler
	handlers       []*rpcServer
}

func NewRPCBuilder() *RPCBuilder {
	return &RPCBuilder{}
}

func (r *RPCBuilder) Conn(c net.Conn) *RPCBuilder {
	r.conn = c
	return r
}

func (r *RPCBuilder) Dialer(d Dialer) *RPCBuilder {
	r.dialer = d
	return r
}

func (r *RPCBuilder) ConnectHook(h ConnectedHandler) *RPCBuilder {
	r.connectHook = h
	return r
}

func (r *RPCBuilder) DisconnectHook(h DisconnectedHandler) *RPCBuilder {
	r.disconnectHook = h
	return r
}

func (r *RPCBuilder) Server(s ...*rpcServer) *RPCBuilder {
	r.handlers = s
	return r
}

func (r *RPCBuilder) Build() *RPCConnection {
	c := &RPCConnection{
		done:         make(chan struct{}),
		dialer:       r.dialer,
		handlers:     make(map[uint64]rpcCallHandler),
		onDisconnect: r.disconnectHook,
		onConnect:    r.connectHook,
		conn:         newConnection(),
	}

	for _, h := range r.handlers {
		c.setHandler(h.ProtocolID, h.Handler)
	}

	c.newRConn()

	if r.conn != nil {
		c.connect(r.conn)
	} else {
		c.dial()
	}
	return c
}

func (c *RPCConnection) newRConn() {
	select {
	case <-c.conn.ready:
		c.conn = newConnection()
	default:
	}
}

func (c *RPCConnection) connect(conn net.Conn) bool {
	c.newRConn()
	c.conn.conn = conn

	if err := c.sendPreamble(c.conn); err != nil {
		c.end(c.conn, err)
		close(c.conn.ready)
		return false
	}

	// TODO(kl): Wait for onConnect to release ready!
	// How, though? Magic context to bypass ready wait?
	close(c.conn.ready)
	go c.readLoop()

	if c.onConnect != nil {
		if err := c.onConnect(c); err != nil {
			c.end(c.conn, &rpcError{id: ApplicationError, error: err.Error()})
			return false
		}
	}

	return true
}

func (c *RPCConnection) dial() {
	select {
	case <-c.done:
		return
	default:
	}

	conn, err := c.dialer()
	if err != nil {
		close(c.done)
		c.end(c.conn, &rpcError{id: ApplicationError, error: err.Error()})
		return
	}
	if c.connect(conn) {
		return
	}
}

func (c *RPCConnection) sendPreamble(conn *connection) RPCError {
	if _, err := c.conn.conn.Write(rpcPreamble); err != nil {
		err := &rpcError{id: ConnectionError, error: fmt.Sprintf("could not write preamble: %v", err)}
		c.end(conn, err)
		return err
	}
	return nil
}

func (c *RPCConnection) readLoop() {
	var (
		receivedPreamble bool
		off, minRead int = 0, 256
		buf = make([]byte, 0, 512)
	)

	ctx, cancel := context.WithCancel(context.Background())

	conn := c.conn

	// kill bad connections
	defer func() {
		cancel()
		if bad := recover(); bad != nil {
			c.end(conn, &rpcError{id: GenericError, error: fmt.Sprintf("unhandled error in readLoop(): %v", bad)})
		}

		if c.dialer != nil {
			c.dial()
		}
	}()

	for {
		// read from the connection
		if off >= len(buf) {
			buf = buf[:0]
			off = 0
		}

		if free := cap(buf) - len(buf); free < minRead {
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

		m, err := conn.conn.Read(buf[len(buf):cap(buf)])
		buf = buf[0 : len(buf)+m]
		if err == io.EOF {
			c.end(conn, nil)
			return
		} else if err != nil {
			c.end(conn, &rpcError{id: GenericError, error: fmt.Sprintf("error reading from connection in readLoop(): %v", err)})
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
						c.end(conn, &rpcError{id: ProtocolError, error: "did not receive expected rpckit2 preamble"})
						return
					}
				} else {
					break
				}
			}

			// parse messages
			msg, length := messageFromBytes(buf[off:])
			if msg != nil {
				off += length
				if err := c.gotMessage(ctx, conn, msg); err != nil {
					c.end(conn, err)
					return
				}
			} else {
				break
			}
		}
	}
}

func (c *RPCConnection) gotMessage(ctx context.Context, conn *connection, msg *message) RPCError {
	messageType, err := msg.ReadVarint()
	if err != nil {
		return &rpcError{id: ProtocolError, error: "could not read message type of incoming message"}
	}

	switch messageType {
	case messageTypeMethodCall:
		callID, err := msg.ReadVarint()
		if err != nil {
			return &rpcError{id: ProtocolError, error: "could not read callID from method call"}
		}
		protocolID, err := msg.ReadVarint()
		if err != nil {
			return &rpcError{id: ProtocolError, error: "could not read protocolID from method call"}
		}
		handler, found := c.handlers[protocolID]
		if !found {
			err := &rpcError{id: ProtocolError, error: fmt.Sprintf("unknown protocol: %d", protocolID)}

			reply := newMessage(messageCapacity)
			reply.WriteVarint(messageTypeMethodReturn)
			reply.WriteVarint(callID)
			reply.WriteVarint(err.RPCID())
			err.RPCEncode(reply)
			c.send(conn, reply)

			return err
		}
		methodID, err := msg.ReadVarint()
		if err != nil {
			return &rpcError{id: ProtocolError, error: "could not read methodID from method call"}
		}

		go func() {
			result := handler.RPCCall(ctx, methodID, msg)
			if result != nil {
				reply := newMessage(messageCapacity)
				reply.WriteVarint(messageTypeMethodReturn)
				reply.WriteVarint(callID)
				reply.WriteVarint(result.RPCID())
				result.RPCEncode(reply)
				if err := c.send(conn, reply); err != nil {
					c.end(conn, err)
				}
			}
		}()

	case messageTypeMethodReturn:
		callID, err := msg.ReadVarint()
		if err != nil {
			return &rpcError{id: ProtocolError, error: "could not read callID from method return"}
		}

		conn.callIDLock.RLock()
		ch, found := conn.waitingCalls[callID]
		conn.callIDLock.RUnlock()
		if !found {
			return &rpcError{id: ProtocolError, error: fmt.Sprintf("method response received for unknown call: %v", callID)}
		}

		ch <- msg
	}

	return nil
}

func contextToError(e error) RPCError {
	switch e {
	case context.DeadlineExceeded:
		return &rpcError{id: TimeoutError, error: "method call timed out"}
	case context.Canceled:
		return &rpcError{id: GenericError, error: "method call cancelled"}
	default:
		return &rpcError{id: GenericError, error: fmt.Sprintf("unknown cancellation reason: %+v", e)}
	}
}

func (c *RPCConnection) call(ctx context.Context, waitForReply bool, protocolID uint64, methodID uint64, callArgs rpcMessage) (uint64, *message, RPCError) {
	conn := c.conn
	select {
	case <-conn.ready:
	case <-ctx.Done():
		return 0, nil, contextToError(ctx.Err())
	}
	if !conn.connected {
		if conn.endReason != nil {
			if err, ok := conn.endReason.(RPCError); ok {
				return 0, nil, err
			}
			return 0, nil, &rpcError{id: GenericError, error: conn.endReason.Error()}
		} else {
			return 0, nil, &rpcError{id: ConnectionError, error: "not connected"}
		}
	}

	callID, err := conn.acquireCallSlot()
	if err != nil {
		return 0, nil, err
	}

	m := newMessage(messageCapacity)
	m.WriteVarint(messageTypeMethodCall)
	m.WriteVarint(callID)
	m.WriteVarint(protocolID)
	m.WriteVarint(methodID)

	if err := callArgs.RPCEncode(m); err != nil {
		conn.abandonCallSlot(callID)
		return 0, nil, &rpcError{id: GenericError, error: fmt.Sprintf("unable to encode method call arguments: %v", err)}
	}

	if !waitForReply {
		conn.abandonCallSlot(callID)
		return 0, nil, c.send(conn, m)
	}

	// setup the call and timer
	ch := make(chan *message, 1)
	conn.callIDLock.Lock()
	conn.waitingCalls[callID] = ch
	conn.callIDLock.Unlock()

	// send the call over to the otherside
	sendErr := c.send(conn, m)
	if sendErr != nil {
		return 0, nil, sendErr
	}

	// wait for the reply
	select {
	case msg := <-ch:
		conn.releaseCallSlot(callID)

		resultTypeID, err := msg.ReadVarint()
		if err != nil {
			err2 := &rpcError{id: GenericError, error: fmt.Sprintf("error while decoding message type: %v", err)}
			c.end(conn, err2)
			return 0, nil, err2
		}
		return resultTypeID, msg, nil
	case <-ctx.Done():
		// We're cancelled. Let a goroutine manage cleanup of the call-slot.
		go func() {
			<-ch
			conn.releaseCallSlot(callID)
		}()

		// TODO(kl): Should we send a hint to the server, to let it cancel
		// operation on this request?
		return 0, nil, contextToError(ctx.Err())
	}
}

func (c *RPCConnection) send(conn *connection, msg *message) RPCError {
	bytes := msg.Bytes()
	conn.writeLock.Lock()
	_, err := conn.conn.Write(bytes)
	conn.writeLock.Unlock()

	if err != nil {
		err2 := &rpcError{id: ConnectionError, error: fmt.Sprintf("error while writing message: %v", err)}
		c.end(conn, err2)
		return err2
	}
	return nil
}

func (c *RPCConnection) Close() {
	close(c.done)
	c.end(c.conn, nil)
}

func (c *RPCConnection) Wait() {
	<-c.done
}

func (c *RPCConnection) Ready() {
	<-c.conn.ready
}

func (c *RPCConnection) end(conn *connection, err RPCError) {
	if err != nil && conn.endReason == nil {
		conn.endReason = err
	}

	if conn.connected {
		conn.connected = false
		conn.conn.Close()
		c.onDisconnect(c, err)
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

func (c *RPCConnection) handlePrivateResponse(resultType uint64, msg *message) (RPCError, bool) {
	if isRPCError(resultType) {
		var r rpcError
		if err := r.RPCDecode(msg); err != nil {
			return &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", err)}, true
		}
		return &r, true
	}
	return nil, false
}

type PingpongMethod uint64

type PingpongClient struct {
	c *RPCConnection
}

func NewPingpongClient(c *RPCConnection) *PingpongClient {
	return &PingpongClient{c: c}
}


const PingpongMethod_Authenticate PingpongMethod = 1

type request_Pingpong_Authenticate struct {
    _username string
    _password string
}

func (s *request_Pingpong_Authenticate) RPCID() uint64 {
    return uint64(PingpongMethod_Authenticate)
}

func (s *request_Pingpong_Authenticate) RPCEncode(m *message) error {
    m.WritePBString(1, s._username)
    m.WritePBString(2, s._password)
    return nil
}

func (s *request_Pingpong_Authenticate) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag {
        case uint64(1 << 3) | uint64(wireTypeLengthDelimited):
            s._username, err = m.ReadString()
        case uint64(2 << 3) | uint64(wireTypeLengthDelimited):
            s._password, err = m.ReadString()
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

type response_Pingpong_Authenticate struct {
    _success bool
}

func (s *response_Pingpong_Authenticate) RPCID() uint64 {
    return uint64(PingpongMethod_Authenticate)
}

func (s *response_Pingpong_Authenticate) RPCEncode(m *message) error {
    m.WritePBBool(1, s._success)
    return nil
}

func (s *response_Pingpong_Authenticate) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag {
        case uint64(1 << 3) | uint64(wireTypeVarint):
            s._success, err = m.ReadBool()
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

func (c *PingpongClient) Authenticate(
    ctx context.Context,
        _username string,
        _password string,
    ) (
        _success bool,
        err RPCError,
    ) {

    resultTypeID, msg, err := c.c.call(ctx, true, 1, uint64(PingpongMethod_Authenticate), &request_Pingpong_Authenticate{
        _username: _username,
        _password: _password,
    })

    if err != nil {
        if rpcErr, ok := err.(RPCError); ok {
            err = rpcErr
        } else {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("call failed: %+v\n", err.Error())}
        }
        return
    }

    switch resultTypeID {
    case uint64(PingpongMethod_Authenticate):
        var r response_Pingpong_Authenticate
        if decodeErr := r.RPCDecode(msg); decodeErr != nil {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", decodeErr)}
            return
        }
        _success = r._success
    default:
        var isPrivate bool
        err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
        if !isPrivate {
            err = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(PingpongMethod_Authenticate), resultTypeID)}
        }
    }

    return
}


const PingpongMethod_PingWithReply PingpongMethod = 2

type request_Pingpong_PingWithReply struct {
    _name string
}

func (s *request_Pingpong_PingWithReply) RPCID() uint64 {
    return uint64(PingpongMethod_PingWithReply)
}

func (s *request_Pingpong_PingWithReply) RPCEncode(m *message) error {
    m.WritePBString(1, s._name)
    return nil
}

func (s *request_Pingpong_PingWithReply) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag {
        case uint64(1 << 3) | uint64(wireTypeLengthDelimited):
            s._name, err = m.ReadString()
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

type response_Pingpong_PingWithReply struct {
    _greeting string
}

func (s *response_Pingpong_PingWithReply) RPCID() uint64 {
    return uint64(PingpongMethod_PingWithReply)
}

func (s *response_Pingpong_PingWithReply) RPCEncode(m *message) error {
    m.WritePBString(1, s._greeting)
    return nil
}

func (s *response_Pingpong_PingWithReply) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag {
        case uint64(1 << 3) | uint64(wireTypeLengthDelimited):
            s._greeting, err = m.ReadString()
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

func (c *PingpongClient) PingWithReply(
    ctx context.Context,
        _name string,
    ) (
        _greeting string,
        err RPCError,
    ) {

    resultTypeID, msg, err := c.c.call(ctx, true, 1, uint64(PingpongMethod_PingWithReply), &request_Pingpong_PingWithReply{
        _name: _name,
    })

    if err != nil {
        if rpcErr, ok := err.(RPCError); ok {
            err = rpcErr
        } else {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("call failed: %+v\n", err.Error())}
        }
        return
    }

    switch resultTypeID {
    case uint64(PingpongMethod_PingWithReply):
        var r response_Pingpong_PingWithReply
        if decodeErr := r.RPCDecode(msg); decodeErr != nil {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", decodeErr)}
            return
        }
        _greeting = r._greeting
    default:
        var isPrivate bool
        err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
        if !isPrivate {
            err = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(PingpongMethod_PingWithReply), resultTypeID)}
        }
    }

    return
}


const PingpongMethod_TestMethod PingpongMethod = 3

type request_Pingpong_TestMethod struct {
    _string string
    _bool bool
    _int64 int64
    _int int64
    _float float32
    _double float64
}

func (s *request_Pingpong_TestMethod) RPCID() uint64 {
    return uint64(PingpongMethod_TestMethod)
}

func (s *request_Pingpong_TestMethod) RPCEncode(m *message) error {
    m.WritePBString(1, s._string)
    m.WritePBBool(2, s._bool)
    m.WritePBInt64(3, s._int64)
    m.WritePBInt(4, s._int)
    m.WritePBFloat(5, s._float)
    m.WritePBDouble(6, s._double)
    return nil
}

func (s *request_Pingpong_TestMethod) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag {
        case uint64(1 << 3) | uint64(wireTypeLengthDelimited):
            s._string, err = m.ReadString()
        case uint64(2 << 3) | uint64(wireTypeVarint):
            s._bool, err = m.ReadBool()
        case uint64(3 << 3) | uint64(wireType64bit):
            s._int64, err = m.ReadInt64()
        case uint64(4 << 3) | uint64(wireTypeVarint):
            s._int, err = m.ReadInt()
        case uint64(5 << 3) | uint64(wireType32bit):
            s._float, err = m.ReadFloat()
        case uint64(6 << 3) | uint64(wireType64bit):
            s._double, err = m.ReadDouble()
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

type response_Pingpong_TestMethod struct {
    _success bool
}

func (s *response_Pingpong_TestMethod) RPCID() uint64 {
    return uint64(PingpongMethod_TestMethod)
}

func (s *response_Pingpong_TestMethod) RPCEncode(m *message) error {
    m.WritePBBool(1, s._success)
    return nil
}

func (s *response_Pingpong_TestMethod) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag {
        case uint64(1 << 3) | uint64(wireTypeVarint):
            s._success, err = m.ReadBool()
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

func (c *PingpongClient) TestMethod(
    ctx context.Context,
        _string string,
        _bool bool,
        _int64 int64,
        _int int64,
        _float float32,
        _double float64,
    ) (
        _success bool,
        err RPCError,
    ) {

    resultTypeID, msg, err := c.c.call(ctx, true, 1, uint64(PingpongMethod_TestMethod), &request_Pingpong_TestMethod{
        _string: _string,
        _bool: _bool,
        _int64: _int64,
        _int: _int,
        _float: _float,
        _double: _double,
    })

    if err != nil {
        if rpcErr, ok := err.(RPCError); ok {
            err = rpcErr
        } else {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("call failed: %+v\n", err.Error())}
        }
        return
    }

    switch resultTypeID {
    case uint64(PingpongMethod_TestMethod):
        var r response_Pingpong_TestMethod
        if decodeErr := r.RPCDecode(msg); decodeErr != nil {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", decodeErr)}
            return
        }
        _success = r._success
    default:
        var isPrivate bool
        err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
        if !isPrivate {
            err = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(PingpongMethod_TestMethod), resultTypeID)}
        }
    }

    return
}


type PingpongMethods interface {
    Authenticate(
    ctx context.Context,
        _username string,
        _password string,
    ) (
        _success bool,
        err error,
    )
    PingWithReply(
    ctx context.Context,
        _name string,
    ) (
        _greeting string,
        err error,
    )
    TestMethod(
    ctx context.Context,
        _string string,
        _bool bool,
        _int64 int64,
        _int int64,
        _float float32,
        _double float64,
    ) (
        _success bool,
        err error,
    )
}

func PingpongHandler(methods PingpongMethods) *rpcServer {
    return &rpcServer{
        ProtocolID: 1,
        Handler: &callHandlerForPingpong{methods: methods},
    }
}

type callHandlerForPingpong struct {
    methods PingpongMethods
}

func (s *callHandlerForPingpong) RPCCall(ctx context.Context, methodID uint64, m *message) (resp rpcMessage) {
    defer func() {
        if r := recover(); r != nil {
            resp = &rpcError{id: ApplicationError, error: "unknown error occurred"}
        }
    }()

    switch methodID {
    case uint64(PingpongMethod_Authenticate):
        args := request_Pingpong_Authenticate{}
        if err := args.RPCDecode(m); err != nil {
            return &rpcError{id: ProtocolError, error: fmt.Sprintf("unable to decode method call: %v", err)}
        }
        success, err := s.methods.Authenticate(
            ctx,
            args._username,
            args._password,
        )
        if err != nil {
            if rpcMsg, ok := err.(rpcMessage); ok {
                return rpcMsg
            }
            return &rpcError{id: ApplicationError, error: err.Error()}
        }
        return &response_Pingpong_Authenticate{
            _success: success,
        }
    case uint64(PingpongMethod_PingWithReply):
        args := request_Pingpong_PingWithReply{}
        if err := args.RPCDecode(m); err != nil {
            return &rpcError{id: ProtocolError, error: fmt.Sprintf("unable to decode method call: %v", err)}
        }
        greeting, err := s.methods.PingWithReply(
            ctx,
            args._name,
        )
        if err != nil {
            if rpcMsg, ok := err.(rpcMessage); ok {
                return rpcMsg
            }
            return &rpcError{id: ApplicationError, error: err.Error()}
        }
        return &response_Pingpong_PingWithReply{
            _greeting: greeting,
        }
    case uint64(PingpongMethod_TestMethod):
        args := request_Pingpong_TestMethod{}
        if err := args.RPCDecode(m); err != nil {
            return &rpcError{id: ProtocolError, error: fmt.Sprintf("unable to decode method call: %v", err)}
        }
        success, err := s.methods.TestMethod(
            ctx,
            args._string,
            args._bool,
            args._int64,
            args._int,
            args._float,
            args._double,
        )
        if err != nil {
            if rpcMsg, ok := err.(rpcMessage); ok {
                return rpcMsg
            }
            return &rpcError{id: ApplicationError, error: err.Error()}
        }
        return &response_Pingpong_TestMethod{
            _success: success,
        }
    default:
        return &rpcError{id: GenericError, error: fmt.Sprintf("unknown method ID: %d", methodID)}
    }
}

type EchoMethod uint64

type EchoClient struct {
	c *RPCConnection
}

func NewEchoClient(c *RPCConnection) *EchoClient {
	return &EchoClient{c: c}
}


const EchoMethod_Echo EchoMethod = 1

type request_Echo_Echo struct {
    _input string
    _names []string
    _values map[string]int64
}

func (s *request_Echo_Echo) RPCID() uint64 {
    return uint64(EchoMethod_Echo)
}

func (s *request_Echo_Echo) RPCEncode(m *message) error {
    m.WritePBString(1, s._input)
    {
        for _, v := range s._names {
            m.WritePBString(2, v)
        }
    }
    {
        for k, v := range s._values {
            em := newEmbeddedMessage(messageCapacity)
            em.WritePBString(1, k)
            em.WritePBInt(2, v)
            m.WritePBMessage(3, em)
        }
    }
    return nil
}

func (s *request_Echo_Echo) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag {
        case uint64(1 << 3) | uint64(wireTypeLengthDelimited):
            s._input, err = m.ReadString()
        case uint64(2 << 3) | uint64(wireTypeLengthDelimited):
            var v string
            v, err = m.ReadString()
            s._names = append(s._names, v)
        case uint64(3 << 3) | uint64(wireTypeLengthDelimited):
            var em *message
            if s._values == nil {
                s._values = make(map[string]int64)
            }

            var k string
            var v int64
            em, err = m.ReadEmbeddedMessage()
            if err != nil {
                break
            }

            tag, err = em.ReadVarint()
            switch tag {
            case uint64(1 << 3) | uint64(wireTypeLengthDelimited):
                k, err = em.ReadString()
            default:
                if err != io.EOF {
                    err = m.ReadPBSkip(tag)
                }
            }

            tag, err = em.ReadVarint()
            switch tag {
            case uint64(2 << 3) | uint64(wireTypeVarint):
                v, err = em.ReadInt()
                s._values[k] = v
            default:
                if err != io.EOF {
                    err = m.ReadPBSkip(tag)
                }
            }
            if err == io.EOF {
                err = nil
            } else if err != nil {
                break
            }
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

type response_Echo_Echo struct {
    _output string
}

func (s *response_Echo_Echo) RPCID() uint64 {
    return uint64(EchoMethod_Echo)
}

func (s *response_Echo_Echo) RPCEncode(m *message) error {
    m.WritePBString(1, s._output)
    return nil
}

func (s *response_Echo_Echo) RPCDecode(m *message) error {
    var (
        err error
        tag uint64
    )
    for err == nil {
        tag, err = m.ReadVarint()
        switch tag {
        case uint64(1 << 3) | uint64(wireTypeLengthDelimited):
            s._output, err = m.ReadString()
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

func (c *EchoClient) Echo(
    ctx context.Context,
        _input string,
        _names []string,
        _values map[string]int64,
    ) (
        _output string,
        err RPCError,
    ) {

    resultTypeID, msg, err := c.c.call(ctx, true, 2, uint64(EchoMethod_Echo), &request_Echo_Echo{
        _input: _input,
        _names: _names,
        _values: _values,
    })

    if err != nil {
        if rpcErr, ok := err.(RPCError); ok {
            err = rpcErr
        } else {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("call failed: %+v\n", err.Error())}
        }
        return
    }

    switch resultTypeID {
    case uint64(EchoMethod_Echo):
        var r response_Echo_Echo
        if decodeErr := r.RPCDecode(msg); decodeErr != nil {
            err = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", decodeErr)}
            return
        }
        _output = r._output
    default:
        var isPrivate bool
        err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
        if !isPrivate {
            err = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(EchoMethod_Echo), resultTypeID)}
        }
    }

    return
}


type EchoMethods interface {
    Echo(
    ctx context.Context,
        _input string,
        _names []string,
        _values map[string]int64,
    ) (
        _output string,
        err error,
    )
}

func EchoHandler(methods EchoMethods) *rpcServer {
    return &rpcServer{
        ProtocolID: 2,
        Handler: &callHandlerForEcho{methods: methods},
    }
}

type callHandlerForEcho struct {
    methods EchoMethods
}

func (s *callHandlerForEcho) RPCCall(ctx context.Context, methodID uint64, m *message) (resp rpcMessage) {
    defer func() {
        if r := recover(); r != nil {
            resp = &rpcError{id: ApplicationError, error: "unknown error occurred"}
        }
    }()

    switch methodID {
    case uint64(EchoMethod_Echo):
        args := request_Echo_Echo{}
        if err := args.RPCDecode(m); err != nil {
            return &rpcError{id: ProtocolError, error: fmt.Sprintf("unable to decode method call: %v", err)}
        }
        output, err := s.methods.Echo(
            ctx,
            args._input,
            args._names,
            args._values,
        )
        if err != nil {
            if rpcMsg, ok := err.(rpcMessage); ok {
                return rpcMsg
            }
            return &rpcError{id: ApplicationError, error: err.Error()}
        }
        return &response_Echo_Echo{
            _output: output,
        }
    default:
        return &rpcError{id: GenericError, error: fmt.Sprintf("unknown method ID: %d", methodID)}
    }
}

