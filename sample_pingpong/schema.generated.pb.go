package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

type rpcContextValue int

const (
	connectionContextKey  rpcContextValue = 1
	alwaysReadyContextKey rpcContextValue = 2
)

func RPCGetConnFromContext(ctx context.Context) net.Conn {
	v := ctx.Value(connectionContextKey)
	if v, ok := v.(net.Conn); ok {
		return v
	}
	return nil
}

// RPC method types.
const (
	messageTypeMethodCall   = 1
	messageTypeMethodReturn = 2
)

// Internal result types.
//
// Negative result types are reserved for internal use. Note that Go does not
// permit negative-to-unsigned casts in constants, hence the use of the
// math.MaxUint64 for -1.
const (
	privateTypeRPCError uint64 = math.MaxUint64 // -1
)

// Default message capacity for messages.
const messageCapacity = 1024

var (
	ErrRPCVarintOverflow = errors.New("overflow in varint")
	ErrGiveUp            = errors.New("give up")
)

// The RPC preamble: "RPCKIT" + 1 as big-endian uint32.
var rpcPreamble = []byte{82, 80, 67, 75, 73, 84, 0, 0, 0, 1}

type rpcMessage interface {
	RPCEncode(m *message) error
	RPCDecode(m *message) error
	RPCID() uint64
}

type rpcCallable interface {
	call(ctx context.Context, h rpcCallServer) rpcMessage
}

type rpcCallServer interface {
	handle(ctx context.Context, methodID uint64, m *message) (rpcCallable, error)
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

func toRPCError(e error) RPCError {
	switch e {
	case nil:
		return nil
	case context.DeadlineExceeded:
		return &rpcError{id: TimeoutError, error: "method call timed out"}
	case context.Canceled:
		return &rpcError{id: GenericError, error: "method call cancelled"}
	}

	switch x := e.(type) {
	case RPCError:
		return x
	}

	return &rpcError{id: GenericError, error: fmt.Sprintf("unknown cancellation reason: %+v", e)}
}

// ************************

// RPCErrorID specifies the error category.
type RPCErrorID uint64

const (
	GenericError     RPCErrorID = 0
	TimeoutError     RPCErrorID = 1
	ProtocolError    RPCErrorID = 2
	ApplicationError RPCErrorID = 3
	ConnectionError  RPCErrorID = 4
	Shutdown         RPCErrorID = 5
)

// RPCError represent RPC errors.
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
		case uint64(1<<3) | uint64(wireTypeVarint):
			var id int64
			id, err = m.ReadInt()
			e.id = RPCErrorID(id)
		case uint64(2<<3) | uint64(wireTypeLengthDelimited):
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

// ************************

// RPCServer represents a protocol handling server implementation.
type RPCServer interface {
	protocolID() uint64
	handler() rpcCallServer
}

type rpcServer struct {
	ProtocolID uint64
	Server     rpcCallServer
}

func (r *rpcServer) protocolID() uint64 {
	return r.ProtocolID
}

func (r *rpcServer) handler() rpcCallServer {
	return r.Server
}

func newRPCServer(protocolID uint64, handler rpcCallServer) *rpcServer {
	return &rpcServer{
		ProtocolID: protocolID,
		Server:     handler,
	}
}

// *************************

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

// ************************

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
		buf:      make([]byte, 4+capacity),
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
				len: length,
				pos: 4,
			}
			return m, length
		}
	}
	return nil, 0
}

func embeddedMessageFromBytes(msg []byte) *message {
	m := &message{
		buf:      msg,
		len:      len(msg),
		embedded: true,
	}
	return m
}

func (m *message) Clone() *message {
	b := make([]byte, m.len-m.pos)
	copy(b, m.buf[m.pos:])
	return &message{
		buf:      b,
		len:      len(b),
		embedded: m.embedded,
	}
}

func (m *message) Bytes() []byte {
	b := m.buf[:m.len]
	if !m.embedded {
		binary.BigEndian.PutUint32(b, uint32(m.len))
	}
	return b
}

func (m *message) grow(needed int) {
	if m.len+needed > len(m.buf) {
		if m.len+needed <= cap(m.buf) {
			// We have plenty of pointlessly unused capacity, so use that.
			m.buf = m.buf[0:cap(m.buf)]
		} else {
			// Not enough space anywhere, we need to allocate.
			buf := makeSlice(2*cap(m.buf) + needed)
			copy(buf, m.buf[0:m.len])
			m.buf = buf
		}
	}
}

func (m *message) ReadBytesNoCopy() ([]byte, error) {
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

func (m *message) ReadBytes() ([]byte, error) {
	a, err := m.ReadBytesNoCopy()
	if err != nil {
		return nil, err
	}
	b := make([]byte, len(a))
	copy(b, a)
	return b, nil
}

func (m *message) ReadString() (string, error) {
	str, err := m.ReadBytesNoCopy()
	return string(str), err
}

func (m *message) ReadEmbeddedMessageNoCopy() (*message, error) {
	b, err := m.ReadBytesNoCopy()
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
				m.LastError = ErrRPCVarintOverflow
				return 0, ErrRPCVarintOverflow
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

	x := binary.LittleEndian.Uint32(m.buf[m.pos : m.pos+4])
	m.pos += 4
	return x, nil
}

func (m *message) Read64Bit() (uint64, error) {
	if m.pos+8 > m.len {
		m.LastError = io.EOF
		return 0, io.EOF
	}

	x := binary.LittleEndian.Uint64(m.buf[m.pos : m.pos+8])
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

func (m *message) WriteBool(value bool) {
	if value {
		m.WriteVarint(1)
	} else {
		m.WriteVarint(0)
	}
}

func (m *message) WriteInt64(value int64) {
	m.Write64Bit(uint64(value))
}

func (m *message) WriteInt(value int64) {
	m.WriteVarint(uint64(value))
}

func (m *message) WriteFloat(value float32) {
	m.Write32Bit(math.Float32bits(value))
}

func (m *message) WriteDouble(value float64) {
	m.Write64Bit(math.Float64bits(value))
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

func (m *message) WritePBBool(fieldNumber uint64, value bool) {
	if value != false {
		m.WritePBTag(fieldNumber, wireTypeVarint)
		m.WriteBool(value)
	}
}

func (m *message) WritePBInt64(fieldNumber uint64, value int64) {
	if value != 0 {
		m.WritePBTag(fieldNumber, wireType64bit)
		m.WriteInt64(value)
	}
}

func (m *message) WritePBInt(fieldNumber uint64, value int64) {
	if value != 0 {
		m.WritePBTag(fieldNumber, wireTypeVarint)
		m.WriteInt(value)
	}
}

func (m *message) WritePBFloat(fieldNumber uint64, value float32) {
	if value != 0 {
		m.WritePBTag(fieldNumber, wireType32bit)
		m.WriteFloat(value)
	}
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

type callSlot struct {
	waiter  chan struct{}
	decoder func(m *message) error
}

// connection provides the network connection context for RPCConnection.
type connection struct {
	// ready signals whether connection setup is completed. Users should
	// always wait for this signal before accessing the struct.
	ready chan struct{}

	// conn is the active connection, if available. It becomes nil on close.
	conn net.Conn

	// connected signals whether the connection is considered open.
	connected bool

	// endReason is the first error that caused the connection to terminate.
	endReason RPCError

	// writeLock is used to avoid output corruption by serializing writes.
	writeLock sync.Mutex

	waitingCallsMutex sync.Mutex
	waitingCalls      map[uint64]*callSlot
	nextID            uint64

	curDelay    time.Duration
	connectTime time.Time
}

func newConnection() *connection {
	return &connection{
		ready:        make(chan struct{}, 0),
		connected:    true,
		waitingCalls: make(map[uint64]*callSlot),
	}
}

// acquireCallSlot allocates a waitingCalls call slot with associated message
// channel for use with a method call.
func (c *connection) acquireCallSlot(f func(m *message) error) (uint64, *callSlot) {
	c.waitingCallsMutex.Lock()
	defer c.waitingCallsMutex.Unlock()

	// Find a slot
	//
	// Our callID space is 2**64 wide, and connection-local. It is large
	// enough to allow us to assume that there are always slots available
	// somewhere. The following becomes an infinite loop if that assumption
	// fails.
	var n uint64
	taken := true
	for taken {
		n = c.nextID
		c.nextID++
		_, taken = c.waitingCalls[n]
	}

	// Take the slot
	cs := &callSlot{
		waiter:  make(chan struct{}, 0),
		decoder: f,
	}
	c.waitingCalls[uint64(n)] = cs
	return uint64(n), cs
}

// findAndReleaseCallSlot finds a waitingCalls call slot, returns the
// associated channel, and releases the call slot. It panics if the slot does
// not exist.
func (c *connection) findAndReleaseCallSlot(callID uint64) *callSlot {
	c.waitingCallsMutex.Lock()
	defer c.waitingCallsMutex.Unlock()

	v, ok := c.waitingCalls[callID]
	if !ok {
		panic("attempted to release free call slot")
	}
	delete(c.waitingCalls, callID)
	return v
}

// releaseAllCallSlots releases all known call slots
func (c *connection) releaseAllCallSlots() {
	c.waitingCallsMutex.Lock()
	defer c.waitingCallsMutex.Unlock()
	for id, slot := range c.waitingCalls {
		close(slot.waiter)
		delete(c.waitingCalls, id)
	}
}

// ************************

// RPCOptions are options to use when creating a new RPCConnection.
type RPCOptions struct {
	// Conn is the initial connection to use for the RPCConnection, bypassing
	// an initial call to Dialer.
	//
	// Note that ConnectHook, if specified, will be called even if the
	// connection was manually specified through Conn.
	Conn net.Conn

	// Dialer establishes a new network connection. If Dialer is not nil,
	// RPCConnection will call it on setup if no connection was provided, and
	// whenever a connection is terminated.
	//
	// Dialer is called repeatedly until either a connection is established,
	// the RPCConnection is closed, or ErrGiveUp is returned as error.
	//
	// If Dialer is nil, the connection will be RPCConnection will terminate
	// on error.
	Dialer func() (net.Conn, error)

	// ConnectHook, if not nil, is called every time a connection is deemed
	// ready to use, after successfully sending an RPC preamble. All RPC calls
	// wait until the connect hook has finished before being allowed to run.
	//
	// It can be used to reestablish state after connection reset, such as by
	// sending authentication messages.
	//
	// ConnectHook is called with a special context that allows RPC calls to be
	// made despite the connection not yet being marked as ready. Failure to
	// use the provided context for RPC calls from within the connect hook will
	// result in a deadlock.
	ConnectHook func(ctx context.Context, c *RPCConnection) error

	// DisconnectHook, if not nil, is called every time a connection is
	// terminated, with the error that caused it to terminate. Reconnect will
	// not be attempted before the disconnect hook has returned.
	DisconnectHook func(ctx context.Context, c *RPCConnection, err RPCError)

	// Servers specify the server implementations that will be used to service
	//
	// If a method call is received for a protocol not on this list, an error
	// reply will be sent.
	Servers []RPCServer

	// ErrorLog specifies an optional error logger.
	ErrorLog func(format string, v ...interface{})

	// PanicHandler is called during a server method panic in order to obtain
	// the error messag eto send the client.
	PanicHandler func(protocol, method string, r interface{}) string

	// CallTimeout defines the timeout for call RPC calls. If zero, no timeout
	// will be set.
	CallTimeout time.Duration

	// InitialRetryDelay sets the first delay value for connection retries.
	InitialRetryDelay time.Duration

	// MaxRetryDelay sets the maximum delay value for connetion retries.
	MaxRetryDelay time.Duration

	// RetryResetDelay sets the delay after which a running connection is
	// deemed successful, after which the exponential retry state will be reset.
	RetryResetDelay time.Duration
}

// RPCConnection is a low-level protobuf-based RPC session.
type RPCConnection struct {
	done         chan struct{}
	servers      map[uint64]rpcCallServer
	dialer       func() (net.Conn, error)
	onConnect    func(ctx context.Context, c *RPCConnection) error
	onDisconnect func(ctx context.Context, c *RPCConnection, err RPCError)

	log          func(format string, v ...interface{})
	panicHandler func(protocol, method string, r interface{}) string
	conn         *connection

	callTimeout time.Duration

	resetDelay time.Duration
	startDelay time.Duration
	maxDelay   time.Duration
}

// NewRPCConnection creates a new RPCConnection.
func NewRPCConnection(options *RPCOptions) *RPCConnection {

	c := &RPCConnection{
		done:         make(chan struct{}),
		dialer:       options.Dialer,
		servers:      make(map[uint64]rpcCallServer),
		onDisconnect: options.DisconnectHook,
		onConnect:    options.ConnectHook,
		conn:         newConnection(),
		log:          options.ErrorLog,
		panicHandler: options.PanicHandler,

		callTimeout: options.CallTimeout,

		resetDelay: options.RetryResetDelay,
		startDelay: options.InitialRetryDelay,
		maxDelay:   options.MaxRetryDelay,
	}

	if c.log == nil {
		c.log = func(string, ...interface{}) {}
	}
	if c.panicHandler == nil {
		c.panicHandler = func(protocol, method string, r interface{}) string {
			c.log("panic in server method %s for protocol %s: %+v", strconv.Quote(method), strconv.Quote(protocol), r)
			return "unknown error occurred"
		}
	}

	if c.resetDelay == 0 {
		c.resetDelay = 5 * time.Second
	}
	if c.startDelay == 0 {
		c.startDelay = 1 * time.Second
	}
	if c.maxDelay == 0 {
		c.maxDelay = 1 * time.Minute
	}

	for _, h := range options.Servers {
		c.servers[h.protocolID()] = h.handler()
	}

	if options.Conn != nil {
		c.connect(options.Conn)
	} else {
		go c.dial()
	}
	return c
}

// isDone is a convenience wrapper for checking if "done" has been asserted.
func (c *RPCConnection) isDone() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// refreshInnerConn renews the inner conenction if necessary.
func (c *RPCConnection) refreshInnerConn() *connection {
	conn := c.conn
	select {
	case <-conn.ready:
		// Ready has already been fired, so the connection has been used.
		// Create a fresh one.
		conn = newConnection()
		conn.curDelay = c.conn.curDelay
		conn.connectTime = time.Now()
		c.conn = conn
		return conn
	default:
		// Ready has not been fired, so we will not replace the connection.
		return conn
	}
}

func (c *RPCConnection) sendPreamble(conn *connection) RPCError {
	if _, err := c.conn.conn.Write(rpcPreamble); err != nil {
		return &rpcError{id: ConnectionError, error: fmt.Sprintf("could not write preamble: %v", err)}
	}
	return nil
}

// connect prepares a new connection based on a successfully dialed net.Conn.
//
// The return value indicates whether the read loop has been started.
func (c *RPCConnection) connect(conn net.Conn) bool {
	cc := c.refreshInnerConn()
	cc.conn = conn

	if err := c.sendPreamble(cc); err != nil {
		conn.Close()
		cc.conn = nil
		return false
	}

	go c.readLoop()

	if c.onConnect != nil {
		go func() {
			// We run the onConnect handler with a magic context to permit traffic
			ctx := context.WithValue(context.Background(), alwaysReadyContextKey, true)
			if err := c.onConnect(ctx, c); err != nil {
				c.end(cc, &rpcError{id: ApplicationError, error: err.Error()})
			}
			// Even if we fail, we have to mark the connection as ready, as it has
			// already been put to use and must not be recycled.
			close(cc.ready)
		}()
	} else {
		close(cc.ready)
	}

	return true
}

func (c *RPCConnection) dial() {
	if c.isDone() {
		return
	}

	cc := c.conn

	if time.Now().After(cc.connectTime.Add(c.resetDelay)) || cc.curDelay == 0 {
		cc.curDelay = c.startDelay
	}

	for {
		conn, err := c.dialer()
		if err == ErrGiveUp {
			close(c.done)
			return
		} else if err == nil && c.connect(conn) {
			// The readloop has been started, which will call us again if
			// necessary.
			return
		}

		// exponential back-off with 25% jitter
		jitter := time.Duration(rand.Int63n(int64(cc.curDelay / 4)))
		select {
		case <-c.done:
			return
		case <-time.After(cc.curDelay + jitter):
			cc.curDelay *= 2
			if cc.curDelay > c.maxDelay {
				cc.curDelay = c.maxDelay
			}
		}
	}
}

// readLoop services incoming traffic on a connection. It is also responsible
// for restarting the dialer on error.
func (c *RPCConnection) readLoop() {
	// We store the connection struct, and pass it around. This is to ensure
	// that we only terminate the *current* connection on error, rather than a
	// freshly established one.
	conn := c.conn

	// Check if the RPCConnection has been closed. We can end up here if the
	// RPCConnection was closed after the dial loop checked but before a new
	// connection was inserted, leading to Close closing an old connection.
	if c.isDone() {
		c.end(conn, &rpcError{id: Shutdown, error: "connection is shutting down"})
		return
	}

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), connectionContextKey, conn.conn))
	defer func() {
		// Clean up on termination
		cancel()
		if bad := recover(); bad != nil {
			c.end(conn, &rpcError{id: GenericError, error: fmt.Sprintf("unhandled error in readLoop(): %v", bad)})
		}

		conn.releaseAllCallSlots()
		if c.onDisconnect != nil {
			c.onDisconnect(context.Background(), c, conn.endReason)
		}

		if c.dialer != nil {
			// Re-dial
			c.dial()
		}
	}()

	// Read preamble
	preambleBuffer := make([]byte, len(rpcPreamble))
	if _, err := io.ReadFull(conn.conn, preambleBuffer); err != nil {
		c.end(conn, &rpcError{id: ProtocolError, error: fmt.Sprintf("could not read preamble: %+v: ", err)})
		return
	} else if !bytes.Equal(rpcPreamble, preambleBuffer) {
		c.end(conn, &rpcError{id: ProtocolError, error: "did not receive expected rpckit2 preamble"})
		return
	}

	var (
		off, minRead int = 0, 256
		buf              = make([]byte, 0, 512)
		n            int
		err          error
	)
	for {
		// Process any errors from last iteration
		if err != nil {
			c.end(conn, &rpcError{id: ConnectionError, error: fmt.Sprintf("error reading from connection in readLoop(): %v", err)})
			return
		}

		if off >= len(buf) {
			// Reset the buffer offset
			buf = buf[:0]
			off = 0
		}

		// Check to see if we have room in our buffer
		if free := cap(buf) - len(buf); free < minRead {
			// We have less than minRead space available
			newBuf := buf

			// Check if we need a bigger buffer, or if cleanup is sufficient.
			if off+free < minRead {
				newBuf = makeSlice(2*cap(buf) + minRead)
			}

			// This either copies to the beginning of our initial buffer, or
			// relocates us to our new buffer.
			copy(newBuf, buf[off:])
			buf = newBuf[:len(buf)-off]
			off = 0
		}

		// Error processing is deferred until next iteration to process any
		// received data.
		n, err = conn.conn.Read(buf[len(buf):cap(buf)])
		buf = buf[0 : len(buf)+n]

		// Process our buffer content
		for {
			msg, length := messageFromBytes(buf[off:])
			if msg == nil {
				// Insufficient data to construct a message.
				break
			}

			off += length
			if err := c.gotMessage(ctx, conn, msg); err != nil {
				c.end(conn, err)
				return
			}
		}
	}
}

// gotMessage handles a received message.
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
		handler, found := c.servers[protocolID]
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

		callable, err := handler.handle(ctx, methodID, msg)
		go func() {
			var result rpcMessage
			if err != nil {
				result = &rpcError{id: ProtocolError, error: err.Error()}
			} else {
				result = func() (resp rpcMessage) {
					defer func() {
						if r := recover(); r != nil {
							func() {
								defer func() {
									if r := recover(); r != nil {
										c.log("panic in panic handler: %+v", r)
										resp = &rpcError{id: ApplicationError, error: "unknown error occurred"}
									}
								}()

								proto := protoMap[protocolID]
								method := proto.methods[methodID]
								resp = &rpcError{id: ApplicationError, error: c.panicHandler(proto.name, method.name, r)}
							}()
						}
					}()

					resp = callable.call(ctx, handler)
					return
				}()
			}

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

		callSlot := conn.findAndReleaseCallSlot(callID)
		if callSlot == nil {
			return &rpcError{id: ProtocolError, error: fmt.Sprintf("method response received for unknown call: %v", callID)}
		}

		if err := callSlot.decoder(msg); err != nil {
			return &rpcError{id: ProtocolError, error: fmt.Sprintf("could not decode response received for call: %+v", err)}
		}

		close(callSlot.waiter)
	}

	return nil
}

// call executes a method call, and if waitForReply is true, waits for its
// completion.
func (c *RPCConnection) call(ctx context.Context, decoder func(m *message) error, waitForReply bool, protocolID, methodID uint64, callArgs rpcMessage) RPCError {
	conn := c.conn

	if c.callTimeout != 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, c.callTimeout)
		defer cancel()
	}

	// Is this an "always ready" context?
	if ctx.Value(alwaysReadyContextKey) == nil {
		// Wait for connection readiness or context cancellation.
		select {
		case <-conn.ready:
		case <-ctx.Done():
			return toRPCError(ctx.Err())
		}
	} else {
		// Wait chekc forcontext cancellation.
		select {
		case <-ctx.Done():
			return toRPCError(ctx.Err())
		default:
		}
	}

	// Connection is ready, but is it still connected?
	if !conn.connected {
		if conn.endReason != nil {
			if err, ok := conn.endReason.(RPCError); ok {
				return err
			}
			return &rpcError{id: GenericError, error: conn.endReason.Error()}
		} else {
			return &rpcError{id: ConnectionError, error: "not connected"}
		}
	}

	// Prepare the message
	callID, callSlot := conn.acquireCallSlot(decoder)
	m := newMessage(messageCapacity)
	m.WriteVarint(messageTypeMethodCall)
	m.WriteVarint(callID)
	m.WriteVarint(protocolID)
	m.WriteVarint(methodID)

	if err := callArgs.RPCEncode(m); err != nil {
		conn.findAndReleaseCallSlot(callID)
		return &rpcError{id: GenericError, error: fmt.Sprintf("unable to encode method call arguments: %v", err)}
	}

	// Send the message
	sendErr := c.send(conn, m)
	if sendErr != nil || !waitForReply {
		conn.findAndReleaseCallSlot(callID)
		return sendErr
	}

	// Wait for reply
	select {
	case <-callSlot.waiter:
		// Return any existing errors
		return toRPCError(conn.endReason)
	case <-ctx.Done():
		// TODO(kl): Should we send a hint to the server, to let it cancel
		// operation on this request?
		return toRPCError(ctx.Err())
	}
}

// send serializes a message and sends it down the pipe.
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

// Close terminates the RPCConnection.
func (c *RPCConnection) Close() {
	close(c.done)
	c.end(c.conn, &rpcError{id: Shutdown, error: "connection is shutting down"})
}

// Wait waits for the RPCConnection to be terminated.
func (c *RPCConnection) Wait() {
	<-c.done
}

// Ready waits for the RPCConnection to either become ready for use, or be
// terminated. Ready returns true if the cause for waking up was that the
// connection became ready.
func (c *RPCConnection) Ready() bool {
	select {
	case <-c.conn.ready:
		return true
	case <-c.done:
		return false
	}
}

// end signals that a connection should be terminated. It stores the error it
// was called with, terminates the connection and calls onDisconnect if
// available.
//
// Only the first call to end on a connection has any effect, so it is safe to
// call multiple times.
func (c *RPCConnection) end(conn *connection, err RPCError) {
	if conn.connected {
		conn.endReason = err
		conn.connected = false
		if conn.conn != nil {
			conn.conn.Close()
		}
	}
}

// handlePrivateResponse is called from individual method response handlers
// for unknown message types to handle private message types.
func (c *RPCConnection) handlePrivateResponse(resultType uint64, msg *message) (err RPCError, isPrivate bool) {
	switch resultType {
	case privateTypeRPCError:
		var r rpcError
		if err := r.RPCDecode(msg); err != nil {
			return &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", err)}, true
		}
		return &r, true
	}
	return nil, false
}

func marshalDateTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func unmarshalDateTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

func marshalUUID(u uuid.UUID) string {
	return string(u[:])
}

func unmarshalUUID(s string) (uuid.UUID, error) {
	return uuid.FromBytes([]byte(s))
}

type protoPingpongMethod uint64
type protoEchoMethod uint64

const (
	protoPingpongMethodAuthenticate  protoPingpongMethod = 1
	protoPingpongMethodPingWithReply protoPingpongMethod = 2
	protoPingpongMethodTestMethod    protoPingpongMethod = 3

	protoEchoMethodEcho     protoEchoMethod = 1
	protoEchoMethodPing     protoEchoMethod = 2
	protoEchoMethodByteTest protoEchoMethod = 3
)

type protoDescriptor struct {
	name    string
	methods map[uint64]methodDescriptor
}

type methodDescriptor struct {
	name string
}

var protoMap = map[uint64]protoDescriptor{
	1: protoDescriptor{
		name: "pingpong",
		methods: map[uint64]methodDescriptor{
			1: methodDescriptor{name: "Authenticate"},
			2: methodDescriptor{name: "PingWithReply"},
			3: methodDescriptor{name: "TestMethod"},
		},
	},
	2: protoDescriptor{
		name: "echo",
		methods: map[uint64]methodDescriptor{
			1: methodDescriptor{name: "Echo"},
			2: methodDescriptor{name: "Ping"},
			3: methodDescriptor{name: "ByteTest"},
		},
	},
}

type rpcMap0 struct {
	Key   string
	Value int64
}

func (s *rpcMap0) RPCEncode(m *message) error {
	m.WritePBString(1, s.Key)
	m.WritePBInt(2, s.Value)
	return nil
}

func (s *rpcMap0) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Key, err = m.ReadString()
		case uint64(2<<3) | uint64(wireTypeVarint):
			s.Value, err = m.ReadInt()
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

type rpcMap1 struct {
	Key   time.Time
	Value time.Time
}

func (s *rpcMap1) RPCEncode(m *message) error {
	m.WritePBString(1, marshalDateTime(s.Key))
	m.WritePBString(2, marshalDateTime(s.Value))
	return nil
}

func (s *rpcMap1) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			{
				var x string
				x, err = m.ReadString()
				if err == nil {
					s.Key, err = unmarshalDateTime(x)
				}
			}
		case uint64(2<<3) | uint64(wireTypeLengthDelimited):
			{
				var x string
				x, err = m.ReadString()
				if err == nil {
					s.Value, err = unmarshalDateTime(x)
				}
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

type rpcGlobalStructEchoThing struct {
	v EchoThing
}

func (s *rpcGlobalStructEchoThing) RPCEncode(m *message) error {
	m.WritePBString(1, s.v.Wee)
	m.WritePBString(2, s.v.Woo)
	for k, v := range s.v.Stuff {
		em := newEmbeddedMessage(messageCapacity)
		vv := rpcMap0{
			Key:   k,
			Value: v,
		}
		if err := vv.RPCEncode(em); err != nil {
			return err
		}
		m.WritePBMessage(3, em)
	}
	m.WritePBString(4, marshalDateTime(s.v.Anothertime))
	for k, v := range s.v.Mydatetimemap {
		em := newEmbeddedMessage(messageCapacity)
		vv := rpcMap1{
			Key:   k,
			Value: v,
		}
		if err := vv.RPCEncode(em); err != nil {
			return err
		}
		m.WritePBMessage(5, em)
	}
	m.WritePBString(6, marshalUUID(s.v.Id))
	return nil
}

func (s *rpcGlobalStructEchoThing) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.v.Wee, err = m.ReadString()

		case uint64(2<<3) | uint64(wireTypeLengthDelimited):
			s.v.Woo, err = m.ReadString()

		case uint64(3<<3) | uint64(wireTypeLengthDelimited):
			var em *message
			if s.v.Stuff == nil {
				s.v.Stuff = make(map[string]int64)
			}

			outer := s.v.Stuff

			var v rpcMap0

			if em, err = m.ReadEmbeddedMessageNoCopy(); err != nil {
				break
			}

			if err = v.RPCDecode(em); err == io.EOF {
				err = nil
			} else if err != nil {
				break
			}

			outer[v.Key] = v.Value

		case uint64(4<<3) | uint64(wireTypeLengthDelimited):
			{
				var x string
				x, err = m.ReadString()
				if err == nil {
					s.v.Anothertime, err = unmarshalDateTime(x)
				}
			}

		case uint64(5<<3) | uint64(wireTypeLengthDelimited):
			var em *message
			if s.v.Mydatetimemap == nil {
				s.v.Mydatetimemap = make(map[time.Time]time.Time)
			}

			outer := s.v.Mydatetimemap

			var v rpcMap1

			if em, err = m.ReadEmbeddedMessageNoCopy(); err != nil {
				break
			}

			if err = v.RPCDecode(em); err == io.EOF {
				err = nil
			} else if err != nil {
				break
			}

			outer[v.Key] = v.Value

		case uint64(6<<3) | uint64(wireTypeLengthDelimited):
			{
				var x string
				x, err = m.ReadString()
				if err == nil {
					s.v.Id, err = unmarshalUUID(x)
				}
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

type rpcMap2 struct {
	Key   string
	Value map[string]int64
}

func (s *rpcMap2) RPCEncode(m *message) error {
	m.WritePBString(1, s.Key)
	for k, v := range s.Value {
		em := newEmbeddedMessage(messageCapacity)
		vv := rpcMap0{
			Key:   k,
			Value: v,
		}
		if err := vv.RPCEncode(em); err != nil {
			return err
		}
		m.WritePBMessage(2, em)
	}
	return nil
}

func (s *rpcMap2) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Key, err = m.ReadString()
		case uint64(2<<3) | uint64(wireTypeLengthDelimited):
			var em *message
			if s.Value == nil {
				s.Value = make(map[string]int64)
			}

			outer := s.Value

			var v rpcMap0

			if em, err = m.ReadEmbeddedMessageNoCopy(); err != nil {
				break
			}

			if err = v.RPCDecode(em); err == io.EOF {
				err = nil
			} else if err != nil {
				break
			}

			outer[v.Key] = v.Value
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

type rpcReqProtoPingpongMethodAuthenticate struct {
	Username string
	Password string
}

func (s *rpcReqProtoPingpongMethodAuthenticate) RPCEncode(m *message) error {
	m.WritePBString(1, s.Username)
	m.WritePBString(2, s.Password)
	return nil
}

func (s *rpcReqProtoPingpongMethodAuthenticate) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Username, err = m.ReadString()

		case uint64(2<<3) | uint64(wireTypeLengthDelimited):
			s.Password, err = m.ReadString()

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

func (s *rpcReqProtoPingpongMethodAuthenticate) RPCID() uint64 {
	return uint64(protoPingpongMethodAuthenticate)
}

type rpcRespProtoPingpongMethodAuthenticate struct {
	Success bool
}

func (s *rpcRespProtoPingpongMethodAuthenticate) RPCEncode(m *message) error {
	m.WritePBBool(1, s.Success)
	return nil
}

func (s *rpcRespProtoPingpongMethodAuthenticate) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeVarint):
			s.Success, err = m.ReadBool()

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

func (s *rpcRespProtoPingpongMethodAuthenticate) RPCID() uint64 {
	return uint64(protoPingpongMethodAuthenticate)
}

type rpcReqProtoPingpongMethodPingWithReply struct {
	Name string
}

func (s *rpcReqProtoPingpongMethodPingWithReply) RPCEncode(m *message) error {
	m.WritePBString(1, s.Name)
	return nil
}

func (s *rpcReqProtoPingpongMethodPingWithReply) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Name, err = m.ReadString()

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

func (s *rpcReqProtoPingpongMethodPingWithReply) RPCID() uint64 {
	return uint64(protoPingpongMethodPingWithReply)
}

type rpcRespProtoPingpongMethodPingWithReply struct {
	Greeting string
}

func (s *rpcRespProtoPingpongMethodPingWithReply) RPCEncode(m *message) error {
	m.WritePBString(1, s.Greeting)
	return nil
}

func (s *rpcRespProtoPingpongMethodPingWithReply) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Greeting, err = m.ReadString()

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

func (s *rpcRespProtoPingpongMethodPingWithReply) RPCID() uint64 {
	return uint64(protoPingpongMethodPingWithReply)
}

type rpcReqProtoPingpongMethodTestMethod struct {
	String   string
	Bool     bool
	Int64    int64
	Int      int64
	Float    float32
	Double   float64
	Datetime time.Time
}

func (s *rpcReqProtoPingpongMethodTestMethod) RPCEncode(m *message) error {
	m.WritePBString(1, s.String)
	m.WritePBBool(2, s.Bool)
	m.WritePBInt64(3, s.Int64)
	m.WritePBInt(4, s.Int)
	m.WritePBFloat(5, s.Float)
	m.WritePBDouble(6, s.Double)
	m.WritePBString(7, marshalDateTime(s.Datetime))
	return nil
}

func (s *rpcReqProtoPingpongMethodTestMethod) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.String, err = m.ReadString()

		case uint64(2<<3) | uint64(wireTypeVarint):
			s.Bool, err = m.ReadBool()

		case uint64(3<<3) | uint64(wireType64bit):
			s.Int64, err = m.ReadInt64()

		case uint64(4<<3) | uint64(wireTypeVarint):
			s.Int, err = m.ReadInt()

		case uint64(5<<3) | uint64(wireType32bit):
			s.Float, err = m.ReadFloat()

		case uint64(6<<3) | uint64(wireType64bit):
			s.Double, err = m.ReadDouble()

		case uint64(7<<3) | uint64(wireTypeLengthDelimited):
			{
				var x string
				x, err = m.ReadString()
				if err == nil {
					s.Datetime, err = unmarshalDateTime(x)
				}
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

func (s *rpcReqProtoPingpongMethodTestMethod) RPCID() uint64 {
	return uint64(protoPingpongMethodTestMethod)
}

type rpcRespProtoPingpongMethodTestMethod struct {
	Success bool
}

func (s *rpcRespProtoPingpongMethodTestMethod) RPCEncode(m *message) error {
	m.WritePBBool(1, s.Success)
	return nil
}

func (s *rpcRespProtoPingpongMethodTestMethod) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeVarint):
			s.Success, err = m.ReadBool()

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

func (s *rpcRespProtoPingpongMethodTestMethod) RPCID() uint64 {
	return uint64(protoPingpongMethodTestMethod)
}

type rpcReqProtoEchoMethodEcho struct {
	Input     string
	Names     []string
	Values    map[string]map[string]int64
	Values2   map[string]int64
	Something EchoThing
	Mytime    time.Time
	Id        uuid.UUID
}

func (s *rpcReqProtoEchoMethodEcho) RPCEncode(m *message) error {
	m.WritePBString(1, s.Input)
	for _, v := range s.Names {
		m.WritePBString(2, v)
	}
	for k, v := range s.Values {
		em := newEmbeddedMessage(messageCapacity)
		vv := rpcMap2{
			Key:   k,
			Value: v,
		}
		if err := vv.RPCEncode(em); err != nil {
			return err
		}
		m.WritePBMessage(3, em)
	}
	for k, v := range s.Values2 {
		em := newEmbeddedMessage(messageCapacity)
		vv := rpcMap0{
			Key:   k,
			Value: v,
		}
		if err := vv.RPCEncode(em); err != nil {
			return err
		}
		m.WritePBMessage(4, em)
	}
	{
		em := newEmbeddedMessage(messageCapacity)
		var vs rpcGlobalStructEchoThing
		vs.v = s.Something
		if err := vs.RPCEncode(em); err != nil {
			return err
		}
		m.WritePBMessage(5, em)
	}
	m.WritePBString(6, marshalDateTime(s.Mytime))
	m.WritePBString(7, marshalUUID(s.Id))
	return nil
}

func (s *rpcReqProtoEchoMethodEcho) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Input, err = m.ReadString()

		case uint64(2<<3) | uint64(wireTypeLengthDelimited):
			var v string
			v, err = m.ReadString()
			s.Names = append(s.Names, v)

		case uint64(3<<3) | uint64(wireTypeLengthDelimited):
			var em *message
			if s.Values == nil {
				s.Values = make(map[string]map[string]int64)
			}

			outer := s.Values

			var v rpcMap2

			if em, err = m.ReadEmbeddedMessageNoCopy(); err != nil {
				break
			}

			if err = v.RPCDecode(em); err == io.EOF {
				err = nil
			} else if err != nil {
				break
			}

			outer[v.Key] = v.Value

		case uint64(4<<3) | uint64(wireTypeLengthDelimited):
			var em *message
			if s.Values2 == nil {
				s.Values2 = make(map[string]int64)
			}

			outer := s.Values2

			var v rpcMap0

			if em, err = m.ReadEmbeddedMessageNoCopy(); err != nil {
				break
			}

			if err = v.RPCDecode(em); err == io.EOF {
				err = nil
			} else if err != nil {
				break
			}

			outer[v.Key] = v.Value

		case uint64(5<<3) | uint64(wireTypeLengthDelimited):
			var em *message

			em, err = m.ReadEmbeddedMessageNoCopy()
			if err != nil {
				break
			}

			var vs rpcGlobalStructEchoThing
			if err = vs.RPCDecode(em); err == io.EOF {
				err = nil
			} else if err != nil {
				break
			}

			s.Something = vs.v

		case uint64(6<<3) | uint64(wireTypeLengthDelimited):
			{
				var x string
				x, err = m.ReadString()
				if err == nil {
					s.Mytime, err = unmarshalDateTime(x)
				}
			}

		case uint64(7<<3) | uint64(wireTypeLengthDelimited):
			{
				var x string
				x, err = m.ReadString()
				if err == nil {
					s.Id, err = unmarshalUUID(x)
				}
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

func (s *rpcReqProtoEchoMethodEcho) RPCID() uint64 {
	return uint64(protoEchoMethodEcho)
}

type rpcRespProtoEchoMethodEcho struct {
	Output    string
	OuputTime time.Time
}

func (s *rpcRespProtoEchoMethodEcho) RPCEncode(m *message) error {
	m.WritePBString(1, s.Output)
	m.WritePBString(2, marshalDateTime(s.OuputTime))
	return nil
}

func (s *rpcRespProtoEchoMethodEcho) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Output, err = m.ReadString()

		case uint64(2<<3) | uint64(wireTypeLengthDelimited):
			{
				var x string
				x, err = m.ReadString()
				if err == nil {
					s.OuputTime, err = unmarshalDateTime(x)
				}
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

func (s *rpcRespProtoEchoMethodEcho) RPCID() uint64 {
	return uint64(protoEchoMethodEcho)
}

type rpcReqProtoEchoMethodPing struct{}

func (s *rpcReqProtoEchoMethodPing) RPCEncode(m *message) error { return nil }
func (s *rpcReqProtoEchoMethodPing) RPCDecode(m *message) error { return nil }
func (s *rpcReqProtoEchoMethodPing) RPCID() uint64 {
	return uint64(protoEchoMethodPing)
}

type rpcRespProtoEchoMethodPing struct {
	Output string
}

func (s *rpcRespProtoEchoMethodPing) RPCEncode(m *message) error {
	m.WritePBString(1, s.Output)
	return nil
}

func (s *rpcRespProtoEchoMethodPing) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Output, err = m.ReadString()

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

func (s *rpcRespProtoEchoMethodPing) RPCID() uint64 {
	return uint64(protoEchoMethodPing)
}

type rpcReqProtoEchoMethodByteTest struct {
	Input []byte
}

func (s *rpcReqProtoEchoMethodByteTest) RPCEncode(m *message) error {
	m.WritePBBytes(1, s.Input)
	return nil
}

func (s *rpcReqProtoEchoMethodByteTest) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Input, err = m.ReadBytes()

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

func (s *rpcReqProtoEchoMethodByteTest) RPCID() uint64 {
	return uint64(protoEchoMethodByteTest)
}

type rpcRespProtoEchoMethodByteTest struct {
	Output []byte
}

func (s *rpcRespProtoEchoMethodByteTest) RPCEncode(m *message) error {
	m.WritePBBytes(1, s.Output)
	return nil
}

func (s *rpcRespProtoEchoMethodByteTest) RPCDecode(m *message) error {
	var (
		err error
		tag uint64
	)
	for err == nil {
		tag, err = m.ReadVarint()
		switch tag {
		case uint64(1<<3) | uint64(wireTypeLengthDelimited):
			s.Output, err = m.ReadBytes()

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

func (s *rpcRespProtoEchoMethodByteTest) RPCID() uint64 {
	return uint64(protoEchoMethodByteTest)
}

type rpcCallServerForPingpong struct {
	methods PingpongProtocol
}

type rpcCallServerForEcho struct {
	methods EchoProtocol
}

// RPCPingpongServer creates a new RPCServer for the pingpong protocol.
func RPCPingpongServer(methods PingpongProtocol) RPCServer {
	return &rpcServer{
		ProtocolID: 1,
		Server:     &rpcCallServerForPingpong{methods: methods},
	}
}

// RPCEchoServer creates a new RPCServer for the echo protocol.
func RPCEchoServer(methods EchoProtocol) RPCServer {
	return &rpcServer{
		ProtocolID: 2,
		Server:     &rpcCallServerForEcho{methods: methods},
	}
}

func (args *rpcReqProtoPingpongMethodAuthenticate) call(ctx context.Context, s rpcCallServer) (resp rpcMessage) {
	success, err := s.(*rpcCallServerForPingpong).methods.Authenticate(ctx, args.Username, args.Password)
	if err != nil {
		if rpcMsg, ok := err.(rpcMessage); ok {
			return rpcMsg
		}
		return &rpcError{id: ApplicationError, error: err.Error()}
	}
	return &rpcRespProtoPingpongMethodAuthenticate{
		Success: success,
	}
}

func (args *rpcReqProtoPingpongMethodPingWithReply) call(ctx context.Context, s rpcCallServer) (resp rpcMessage) {
	greeting, err := s.(*rpcCallServerForPingpong).methods.PingWithReply(ctx, args.Name)
	if err != nil {
		if rpcMsg, ok := err.(rpcMessage); ok {
			return rpcMsg
		}
		return &rpcError{id: ApplicationError, error: err.Error()}
	}
	return &rpcRespProtoPingpongMethodPingWithReply{
		Greeting: greeting,
	}
}

func (args *rpcReqProtoPingpongMethodTestMethod) call(ctx context.Context, s rpcCallServer) (resp rpcMessage) {
	success, err := s.(*rpcCallServerForPingpong).methods.TestMethod(ctx, args.String, args.Bool, args.Int64, args.Int, args.Float, args.Double, args.Datetime)
	if err != nil {
		if rpcMsg, ok := err.(rpcMessage); ok {
			return rpcMsg
		}
		return &rpcError{id: ApplicationError, error: err.Error()}
	}
	return &rpcRespProtoPingpongMethodTestMethod{
		Success: success,
	}
}

func (s *rpcCallServerForPingpong) handle(ctx context.Context, methodID uint64, m *message) (callable rpcCallable, err error) {
	switch methodID {
	case uint64(protoPingpongMethodAuthenticate):
		args := rpcReqProtoPingpongMethodAuthenticate{}
		if err := args.RPCDecode(m); err != nil {
			return nil, fmt.Errorf("unable to decode method call: %v", err)
		}
		return &args, nil
	case uint64(protoPingpongMethodPingWithReply):
		args := rpcReqProtoPingpongMethodPingWithReply{}
		if err := args.RPCDecode(m); err != nil {
			return nil, fmt.Errorf("unable to decode method call: %v", err)
		}
		return &args, nil
	case uint64(protoPingpongMethodTestMethod):
		args := rpcReqProtoPingpongMethodTestMethod{}
		if err := args.RPCDecode(m); err != nil {
			return nil, fmt.Errorf("unable to decode method call: %v", err)
		}
		return &args, nil
	default:
		return nil, fmt.Errorf("unknown method ID: %d", methodID)
	}
}

func (args *rpcReqProtoEchoMethodEcho) call(ctx context.Context, s rpcCallServer) (resp rpcMessage) {
	output, ouputTime, err := s.(*rpcCallServerForEcho).methods.Echo(ctx, args.Input, args.Names, args.Values, args.Values2, args.Something, args.Mytime, args.Id)
	if err != nil {
		if rpcMsg, ok := err.(rpcMessage); ok {
			return rpcMsg
		}
		return &rpcError{id: ApplicationError, error: err.Error()}
	}
	return &rpcRespProtoEchoMethodEcho{
		Output:    output,
		OuputTime: ouputTime,
	}
}

func (args *rpcReqProtoEchoMethodPing) call(ctx context.Context, s rpcCallServer) (resp rpcMessage) {
	output, err := s.(*rpcCallServerForEcho).methods.Ping(ctx)
	if err != nil {
		if rpcMsg, ok := err.(rpcMessage); ok {
			return rpcMsg
		}
		return &rpcError{id: ApplicationError, error: err.Error()}
	}
	return &rpcRespProtoEchoMethodPing{
		Output: output,
	}
}

func (args *rpcReqProtoEchoMethodByteTest) call(ctx context.Context, s rpcCallServer) (resp rpcMessage) {
	output, err := s.(*rpcCallServerForEcho).methods.ByteTest(ctx, args.Input)
	if err != nil {
		if rpcMsg, ok := err.(rpcMessage); ok {
			return rpcMsg
		}
		return &rpcError{id: ApplicationError, error: err.Error()}
	}
	return &rpcRespProtoEchoMethodByteTest{
		Output: output,
	}
}

func (s *rpcCallServerForEcho) handle(ctx context.Context, methodID uint64, m *message) (callable rpcCallable, err error) {
	switch methodID {
	case uint64(protoEchoMethodEcho):
		args := rpcReqProtoEchoMethodEcho{}
		if err := args.RPCDecode(m); err != nil {
			return nil, fmt.Errorf("unable to decode method call: %v", err)
		}
		return &args, nil
	case uint64(protoEchoMethodPing):
		args := rpcReqProtoEchoMethodPing{}
		if err := args.RPCDecode(m); err != nil {
			return nil, fmt.Errorf("unable to decode method call: %v", err)
		}
		return &args, nil
	case uint64(protoEchoMethodByteTest):
		args := rpcReqProtoEchoMethodByteTest{}
		if err := args.RPCDecode(m); err != nil {
			return nil, fmt.Errorf("unable to decode method call: %v", err)
		}
		return &args, nil
	default:
		return nil, fmt.Errorf("unknown method ID: %d", methodID)
	}
}

// The RPCPingpongClient type is a RPC client for the pingpong protocol.
type RPCPingpongClient struct {
	c *RPCConnection
}

// NewRPCPingpongClient creates a new RPC client for the pingpong protocol.
func NewRPCPingpongClient(c *RPCConnection) *RPCPingpongClient {
	return &RPCPingpongClient{c: c}
}

// The RPCEchoClient type is a RPC client for the echo protocol.
type RPCEchoClient struct {
	c *RPCConnection
}

// NewRPCEchoClient creates a new RPC client for the echo protocol.
func NewRPCEchoClient(c *RPCConnection) *RPCEchoClient {
	return &RPCEchoClient{c: c}
}

// Authenticate using username and password
func (c *RPCPingpongClient) Authenticate(ctx context.Context, reqUsername string, reqPassword string) (respSuccess bool, err error) {

	var decoderErr error
	decoder := func(msg *message) error {
		resultTypeID, err := msg.ReadVarint()
		if err != nil {
			decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("error while decoding message type: %v", err)}
			return decoderErr
		}
		switch resultTypeID {
		case uint64(protoPingpongMethodAuthenticate):
			var r rpcRespProtoPingpongMethodAuthenticate
			if err := r.RPCDecode(msg); err != nil {
				decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", err)}
				return decoderErr
			}
			respSuccess = r.Success
			return nil
		default:
			var isPrivate bool
			err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
			if !isPrivate {
				decoderErr = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(protoPingpongMethodAuthenticate), resultTypeID)}
				return decoderErr
			}
			decoderErr = err
			return nil
		}
	}

	err = c.c.call(ctx, decoder, true, 1, uint64(protoPingpongMethodAuthenticate), &rpcReqProtoPingpongMethodAuthenticate{
		Username: reqUsername,
		Password: reqPassword,
	})

	if decoderErr != nil {
		err = decoderErr
	}

	return
}

// PingWithReply replies with a greeting based on the provided name
func (c *RPCPingpongClient) PingWithReply(ctx context.Context, reqName string) (respGreeting string, err error) {

	var decoderErr error
	decoder := func(msg *message) error {
		resultTypeID, err := msg.ReadVarint()
		if err != nil {
			decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("error while decoding message type: %v", err)}
			return decoderErr
		}
		switch resultTypeID {
		case uint64(protoPingpongMethodPingWithReply):
			var r rpcRespProtoPingpongMethodPingWithReply
			if err := r.RPCDecode(msg); err != nil {
				decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", err)}
				return decoderErr
			}
			respGreeting = r.Greeting
			return nil
		default:
			var isPrivate bool
			err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
			if !isPrivate {
				decoderErr = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(protoPingpongMethodPingWithReply), resultTypeID)}
				return decoderErr
			}
			decoderErr = err
			return nil
		}
	}

	err = c.c.call(ctx, decoder, true, 1, uint64(protoPingpongMethodPingWithReply), &rpcReqProtoPingpongMethodPingWithReply{
		Name: reqName,
	})

	if decoderErr != nil {
		err = decoderErr
	}

	return
}

// TestMethod is a simple type test
func (c *RPCPingpongClient) TestMethod(ctx context.Context, reqString string, reqBool bool, reqInt64 int64, reqInt int64, reqFloat float32, reqDouble float64, reqDatetime time.Time) (respSuccess bool, err error) {

	var decoderErr error
	decoder := func(msg *message) error {
		resultTypeID, err := msg.ReadVarint()
		if err != nil {
			decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("error while decoding message type: %v", err)}
			return decoderErr
		}
		switch resultTypeID {
		case uint64(protoPingpongMethodTestMethod):
			var r rpcRespProtoPingpongMethodTestMethod
			if err := r.RPCDecode(msg); err != nil {
				decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", err)}
				return decoderErr
			}
			respSuccess = r.Success
			return nil
		default:
			var isPrivate bool
			err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
			if !isPrivate {
				decoderErr = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(protoPingpongMethodTestMethod), resultTypeID)}
				return decoderErr
			}
			decoderErr = err
			return nil
		}
	}

	err = c.c.call(ctx, decoder, true, 1, uint64(protoPingpongMethodTestMethod), &rpcReqProtoPingpongMethodTestMethod{
		String:   reqString,
		Bool:     reqBool,
		Int64:    reqInt64,
		Int:      reqInt,
		Float:    reqFloat,
		Double:   reqDouble,
		Datetime: reqDatetime,
	})

	if decoderErr != nil {
		err = decoderErr
	}

	return
}

// Echo is yet another type test
func (c *RPCEchoClient) Echo(ctx context.Context, reqInput string, reqNames []string, reqValues map[string]map[string]int64, reqValues2 map[string]int64, reqSomething EchoThing, reqMytime time.Time, reqId uuid.UUID) (respOutput string, respOuputTime time.Time, err error) {

	var decoderErr error
	decoder := func(msg *message) error {
		resultTypeID, err := msg.ReadVarint()
		if err != nil {
			decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("error while decoding message type: %v", err)}
			return decoderErr
		}
		switch resultTypeID {
		case uint64(protoEchoMethodEcho):
			var r rpcRespProtoEchoMethodEcho
			if err := r.RPCDecode(msg); err != nil {
				decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", err)}
				return decoderErr
			}
			respOutput = r.Output
			respOuputTime = r.OuputTime
			return nil
		default:
			var isPrivate bool
			err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
			if !isPrivate {
				decoderErr = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(protoEchoMethodEcho), resultTypeID)}
				return decoderErr
			}
			decoderErr = err
			return nil
		}
	}

	err = c.c.call(ctx, decoder, true, 2, uint64(protoEchoMethodEcho), &rpcReqProtoEchoMethodEcho{
		Input:     reqInput,
		Names:     reqNames,
		Values:    reqValues,
		Values2:   reqValues2,
		Something: reqSomething,
		Mytime:    reqMytime,
		Id:        reqId,
	})

	if decoderErr != nil {
		err = decoderErr
	}

	return
}

// Ping is a simple no-input test
func (c *RPCEchoClient) Ping(ctx context.Context) (respOutput string, err error) {

	var decoderErr error
	decoder := func(msg *message) error {
		resultTypeID, err := msg.ReadVarint()
		if err != nil {
			decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("error while decoding message type: %v", err)}
			return decoderErr
		}
		switch resultTypeID {
		case uint64(protoEchoMethodPing):
			var r rpcRespProtoEchoMethodPing
			if err := r.RPCDecode(msg); err != nil {
				decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", err)}
				return decoderErr
			}
			respOutput = r.Output
			return nil
		default:
			var isPrivate bool
			err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
			if !isPrivate {
				decoderErr = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(protoEchoMethodPing), resultTypeID)}
				return decoderErr
			}
			decoderErr = err
			return nil
		}
	}

	err = c.c.call(ctx, decoder, true, 2, uint64(protoEchoMethodPing), &rpcReqProtoEchoMethodPing{})

	if decoderErr != nil {
		err = decoderErr
	}

	return
}

// ByteTest is a byte test
func (c *RPCEchoClient) ByteTest(ctx context.Context, reqInput []byte) (respOutput []byte, err error) {

	var decoderErr error
	decoder := func(msg *message) error {
		resultTypeID, err := msg.ReadVarint()
		if err != nil {
			decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("error while decoding message type: %v", err)}
			return decoderErr
		}
		switch resultTypeID {
		case uint64(protoEchoMethodByteTest):
			var r rpcRespProtoEchoMethodByteTest
			if err := r.RPCDecode(msg); err != nil {
				decoderErr = &rpcError{id: GenericError, error: fmt.Sprintf("could not decode result: %v", err)}
				return decoderErr
			}
			respOutput = r.Output
			return nil
		default:
			var isPrivate bool
			err, isPrivate = c.c.handlePrivateResponse(resultTypeID, msg)
			if !isPrivate {
				decoderErr = &rpcError{id: ProtocolError, error: fmt.Sprintf("unexpected return type for call type %d: %d", uint64(protoEchoMethodByteTest), resultTypeID)}
				return decoderErr
			}
			decoderErr = err
			return nil
		}
	}

	err = c.c.call(ctx, decoder, true, 2, uint64(protoEchoMethodByteTest), &rpcReqProtoEchoMethodByteTest{
		Input: reqInput,
	})

	if decoderErr != nil {
		err = decoderErr
	}

	return
}
