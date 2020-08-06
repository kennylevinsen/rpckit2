import Foundation

enum RPCError: Error {
	case eof
	case protocolError
	case applicationError(String)
}

enum RPCErrorType: UInt64 {
	case generic = 0
	case timeout = 1
	case proto = 2
	case application = 3
	case connection = 4
	case shutdown = 5
}

enum RPCWireType: UInt64 {
	case varInt = 0
	case fixed64Bit = 1
	case lengthDelimited = 2
	case fixed32Bit = 5
}

enum RPCMessageType: UInt64 {
	case methodCall = 1
	case methodReturn = 2
}

class readableMessage {
	var pos: Int = 0
	var len: Int = 0
	var buf: ArraySlice<UInt8>

	init?(slice: ArraySlice<UInt8>) {
		if (slice.count <= 4) {
			return nil
		}
		self.buf = slice
		self.pos = slice.startIndex + 4
		self.len = Int( Data(slice[slice.startIndex..<slice.startIndex+4]).withUnsafeBytes { $0.load(as: UInt32.self) }.bigEndian )
	}

	init(embeddedSlice: ArraySlice<UInt8>) {
		self.buf = embeddedSlice
		self.len = self.buf.startIndex + buf.count
		self.pos = self.buf.startIndex
	}

	func readBytes() -> Result<ArraySlice<UInt8>, RPCError> {
		var len: Int
		switch self.readVarUInt() {
		case .success(let val):
			len = Int(val)
		case .failure(let error):
			return .failure(error)
		}

		if self.pos + len > self.len {
			return .failure(RPCError.eof)
		}
		let slice = self.buf[self.pos ..< self.pos + len]
		self.pos += len
		return .success(slice)
	}

	func readString() -> Result<String, RPCError> {
		switch self.readBytes() {
		case .success(let val):
			if let string = String(data: Data(val), encoding: .utf8) {
				return .success(string)
			} else {
				return .failure(RPCError.protocolError)
			}
		case .failure(let error):
			return .failure(error)
		}
	}

	func readEmbeddedMessage() -> Result<readableMessage, RPCError> {
		switch self.readBytes() {
		case .success(let val):
			return .success(readableMessage(embeddedSlice: val))
		case .failure(let error):
			return .failure(error)
		}
	}

	func readVarUInt() -> Result<UInt64, RPCError> {
		var x: UInt64 = 0
		var s: UInt = 0
		for index in 0..<self.len {
			if self.pos+1 > self.len {
				return .failure(RPCError.eof)
			}
			let b = self.buf[self.pos]
			self.pos += 1
			if b < 0x80 {
				if index > 9 || (index == 9 && b > 1) {
					return .failure(RPCError.protocolError)
				}
				return .success(x | UInt64(b) << s)
			}
			x |= UInt64(b & 0x7F) << s
			s += 7
		}
		return .failure(RPCError.protocolError)
	}

	func readVarInt() -> Result<Int64, RPCError> {
		switch self.readVarUInt() {
		case .success(let val):
			return .success(Int64(val))
		case .failure(let error):
			return .failure(error)
		}
	}

	func readInt() -> Result<Int64, RPCError> {
		return self.readVarInt()
	}

	func readUInt32() -> Result<UInt32, RPCError> {
		if self.pos + 4 > self.len {
			return .failure(RPCError.eof)
		}
		let val = Data(self.buf[self.pos..<self.pos+4]).withUnsafeBytes { $0.load(as: UInt32.self) }
		self.pos += 4
		return .success(val)
	}

	func readUInt64() -> Result<UInt64, RPCError> {
		if self.pos + 8 > self.len {
			return .failure(RPCError.eof)
		}
		let val = Data(self.buf[self.pos..<self.pos+8]).withUnsafeBytes { $0.load(as: UInt64.self) }
		self.pos += 8
		return .success(val)
	}

	func readInt32() -> Result<Int32, RPCError> {
		switch self.readUInt32() {
		case .success(let val):
			return .success(Int32(val))
		case .failure(let error):
			return .failure(error)
		}
	}

	func readInt64() -> Result<Int64, RPCError> {
		switch self.readUInt64() {
		case .success(let val):
			return .success(Int64(val))
		case .failure(let error):
			return .failure(error)
		}
	}

	func readBool() -> Result<Bool, RPCError> {
		switch self.readVarUInt() {
		case .success(let val):
			return .success(val == 1)
		case .failure(let error):
			return .failure(error)
		}
	}

	func readFloat() -> Result<Float, RPCError> {
		if self.pos + 4 > self.len {
			return .failure(RPCError.eof)
		}
		let val = Data(self.buf[self.pos..<self.pos+4]).withUnsafeBytes { $0.load(as: Float.self) }
		self.pos += 4
		return .success(val)
	}

	func readDouble() -> Result<Double, RPCError> {
		if self.pos + 8 > self.len {
			return .failure(RPCError.eof)
		}
		let val = Data(self.buf[self.pos..<self.pos+8]).withUnsafeBytes { $0.load(as: Double.self) }
		self.pos += 8
		return .success(val)
	}

	func readPBSkip(tag: UInt64) -> Result<Void, RPCError> {
		guard let t = RPCWireType(rawValue: tag & ((1 << 3) - 1)) else {
			return .failure(RPCError.protocolError)
		}
		switch t {
		case RPCWireType.varInt:
			switch self.readVarUInt() {
			case .success(_):
				return .success(())
			case .failure(let error):
				return .failure(error)
			}
		case RPCWireType.fixed64Bit:
			switch self.readInt64() {
			case .success(_):
				return .success(())
			case .failure(let error):
				return .failure(error)
			}
		case RPCWireType.lengthDelimited:
			switch self.readBytes() {
			case .success(_):
				return .success(())
			case .failure(let error):
				return .failure(error)
			}
		case RPCWireType.fixed32Bit:
			switch self.readInt32() {
			case .success(_):
				return .success(())
			case .failure(let error):
				return .failure(error)
			}
		}
	}
}

class writableMessage {
	var buf: [UInt8] = []
	var embedded: Bool = false

	init() {
		self.buf.reserveCapacity(1024)
		for _ in 0..<4 {
			self.buf.append(0)
		}
	}

	init(embedded: Bool) {
		self.buf.reserveCapacity(1024)
		if !embedded {
			for _ in 0..<4 {
				self.buf.append(0)
			}
		}
	}

	init(capacity: Int) {
		self.buf.reserveCapacity(capacity)
		for _ in 0..<4 {
			self.buf.append(0)
		}
	}

	init(embeddedCapacity: Int) {
		self.buf.reserveCapacity(embeddedCapacity)
		self.embedded = true
	}

	func finish() {
		if !self.embedded {
			let data = Swift.withUnsafeBytes(of: UInt32(self.buf.count).bigEndian) { Data($0) }
			for idx in 0..<data.count {
				self.buf[idx] = data[idx]
			}
		}
	}

	func writeVarUInt(value: UInt64) {
		var v = value
		while v >= 0x80 {
			self.buf.append(UInt8(v) | 0x80)
			v >>= 7
		}
		self.buf.append(UInt8(v))
	}

	func writeVarInt(value: Int64) {
		return self.writeVarUInt(value: UInt64(value))
	}

	func writeBytes(bytes: ArraySlice<UInt8>) {
		self.writeVarUInt(value: UInt64(bytes.count))
		for byte in bytes {
			self.buf.append(byte)
		}
	}

	func writeString(str: String) {
		let utf8 = str.utf8
		self.writeVarUInt(value: UInt64(utf8.count))
		for byte in utf8 {
			self.buf.append(byte)
		}
	}

	func writeSimple<T>(value: T) {
		let data = Swift.withUnsafeBytes(of: value) { Data($0) }
		for byte in data {
			self.buf.append(byte)
		}
	}

	func writePBTag(fieldNumber: UInt64, wt: RPCWireType) {
		self.writeVarUInt(value: (fieldNumber << 3) | wt.rawValue)
	}

	func writePBString(fieldNumber: UInt64, value: String) {
		if value.count > 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.lengthDelimited)
			self.writeString(str: value)
		}
	}

	func writePBBytes(fieldNumber: UInt64, value: ArraySlice<UInt8>) {
		if value.count > 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.lengthDelimited)
			self.writeBytes(bytes: value)
		}
	}

	func writePBBool(fieldNumber: UInt64, value: Bool) {
		if value {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.varInt)
			self.writeVarUInt(value: 1)
		}
	}

	func writePBVarUInt(fieldNumber: UInt64, value: UInt64) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.varInt)
			self.writeVarUInt(value: value)
		}
	}

	func writePBVarInt(fieldNumber: UInt64, value: Int64) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.varInt)
			self.writeVarInt(value: value)
		}
	}

	func writePBInt(fieldNumber: UInt64, value: Int64) {
		return self.writePBVarInt(fieldNumber: fieldNumber, value: value)
	}

	func writePBInt64(fieldNumber: UInt64, value: Int64) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.fixed64Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBUInt64(fieldNumber: UInt64, value: UInt64) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.fixed64Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBInt32(fieldNumber: UInt64, value: Int32) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.fixed32Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBUInt64(fieldNumber: UInt64, value: UInt32) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.fixed32Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBFloat(fieldNumber: UInt64, value: Float) {
		if value != 0.0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.fixed32Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBDouble(fieldNumber: UInt64, value: Double) {
		if value != 0.0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: RPCWireType.fixed64Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBMessage(fieldNumber: UInt64, msg: writableMessage) {
		self.writePBBytes(fieldNumber: fieldNumber, value: msg.buf[...])
	}
}

protocol RPCMessage {
	func encode(_: writableMessage) -> Result<Void, RPCError>
	static func decode(_: readableMessage) -> Result<RPCMessage, RPCError>
	func id() -> UInt64
}

protocol RPCCallServer {
	func id() -> UInt64
	func handle(methodID: UInt64, rmsg: readableMessage) -> Result<RPCMessage, RPCError>;
}

fileprivate class callSlot {
	let callback: (readableMessage) -> Result<Void, RPCError>

	init(callback: @escaping (readableMessage) -> Result<Void, RPCError>) {
		self.callback = callback
	}
}

fileprivate enum rpcConnectionState {
	case disconnected
	case connecting
	case connected
}

fileprivate enum rpcReaderState {
	case readingPreamble
	case readingHeader
	case readingBody
}

class RPCConnection: NSObject, StreamDelegate {
	// Protocol state
	private var nextID: UInt64 = 0
	private var slots: [UInt64: callSlot] = [:]

	// Immutable paramters
	private let host: String
	private let port: Int
	private let tls: Bool
	private let servers: [UInt64 : RPCCallServer]

	// Connection
	private var connectionState: rpcConnectionState
	private var inputStream: InputStream!
	private var outputStream: OutputStream!

	private var readState: rpcReaderState
	private var readNeeds: Int
	private var readPos: Int
	private var readBuffer: [UInt8]

	private var writeBuffer: [UInt8]

	init(host: String, port: Int, tls: Bool, servers: [RPCCallServer]) {
		self.host = host
		self.port = port
		self.tls = tls
		self.connectionState = rpcConnectionState.disconnected

		self.readState = rpcReaderState.readingPreamble
		self.readNeeds = 10
		self.readPos = 0
		self.readBuffer = Array(repeating: 0, count: 1024)
		self.writeBuffer = []
		var smap: [UInt64 : RPCCallServer] = [:]
		for server in servers {
			smap[server.id()] = server
		}
		self.servers = smap
	}


	convenience init(host: String, port: Int, tls: Bool) {
		self.init(host: host, port: port, tls: tls, servers: [])
	}

	func connect() {
		if self.connectionState != .disconnected {
			return
		}

		self.writeBuffer = [82, 80, 67, 75, 73, 84, 0, 0, 0, 1]
		self.writeBuffer.reserveCapacity(1024)
		self.connectionState = .connecting
		self.readState = rpcReaderState.readingPreamble
		self.readNeeds = 10

		var readStream: Unmanaged<CFReadStream>?
		var writeStream: Unmanaged<CFWriteStream>?

		CFStreamCreatePairWithSocketToHost(kCFAllocatorDefault,
										   self.host as CFString,
										   UInt32(self.port),
										   &readStream,
										   &writeStream)

		self.inputStream = readStream!.takeRetainedValue()
		self.outputStream = writeStream!.takeRetainedValue()

		if self.tls {
			// Enable SSL/TLS on the streams
			self.inputStream!.setProperty(kCFStreamSocketSecurityLevelNegotiatedSSL,
					forKey: Stream.PropertyKey.socketSecurityLevelKey)
			self.outputStream!.setProperty(kCFStreamSocketSecurityLevelNegotiatedSSL,
					forKey: Stream.PropertyKey.socketSecurityLevelKey)

			// Set the SSL/TLS settingson the streams
			let sslSettings: [NSString: Any] = [NSString(format: kCFStreamSSLIsServer): kCFBooleanFalse as Any]
			self.inputStream!.setProperty(sslSettings,
					forKey: kCFStreamPropertySSLSettings as Stream.PropertyKey)
			self.outputStream!.setProperty(sslSettings,
					forKey: kCFStreamPropertySSLSettings as Stream.PropertyKey)
		}

		self.inputStream.delegate = self
		self.outputStream.delegate = self

		self.inputStream.schedule(in: .main, forMode: .common)
		self.outputStream.schedule(in: .main, forMode: .common)

		self.inputStream.open()
		self.outputStream.open()
	}

	func disconnect() {
		if self.connectionState == .connected || self.connectionState == .connecting {
			self.connectionState = .disconnected
			self.inputStream.remove(from: .main, forMode: .common)
			self.outputStream.remove(from: .main, forMode: .common)
			self.inputStream.close()
			self.outputStream.close()
			self.inputStream.delegate = nil
			self.outputStream.delegate = nil
			self.inputStream = nil
			self.outputStream = nil
			self.writeBuffer.removeAll()
		}
		DispatchQueue.main.asyncAfter(deadline: .now() + 5.0, execute: {
			self.connect()
		})
	}

	fileprivate func acquireCallSlot(callback: @escaping (readableMessage) -> Result<Void, RPCError>) -> UInt64 {
		var n: UInt64 = 0
		var taken = true
		while taken {
			n = self.nextID
			self.nextID += 1
			taken = self.slots[n] != nil
		}
		let slot = callSlot(callback: callback)
		self.slots[n] = slot
		return n
	}

	fileprivate func findAndReleaseCallSlot(id: UInt64) -> callSlot? {
		return self.slots.removeValue(forKey: id)
	}

	fileprivate func send(wmsg: writableMessage) {
		self.writeBuffer.append(contentsOf: wmsg.buf)
		self.flushWriteBuffer(stream: self.outputStream)
	}

	fileprivate func hasBytesAvailable(stream: InputStream) {
		// This routine implements a simple, singular message buffer. First a
		// header is read, then the body, then the positions are cleared.
		//
		// This design is simple, but causes excessive read calls. The most
		// efficient design by far would be a ring-buffer, possibly with
		// scaling for when a message that would exceed the buffer space is
		// detected. In second-place is a big-buffer design where data is moved
		// back every time a message is handled.
		//
		// Anyway, back to the video.
		while stream.hasBytesAvailable {
			while self.readPos + self.readNeeds > self.readBuffer.count {
				// This is a dumb way to grow an array.
				self.readBuffer.append(contentsOf: Data(repeating: 0, count: self.readBuffer.count))
			}

			let read = self.readBuffer.withUnsafeMutableBytes { ptr -> Int in
				let posptr = ptr.baseAddress!             // Get a raw pointer
					.assumingMemoryBound(to: UInt8.self)  // Make it a UInt8 pointer
					.advanced(by: self.readPos)           // Move it self.readPos bytes forward
				return stream.read(posptr, maxLength: self.readNeeds - self.readPos)
			}

			self.readPos += read

			if self.readPos < self.readNeeds {
				continue
			}

			switch self.readState {
			case .readingPreamble:
				self.readPos = 0
				self.readNeeds = 4
				self.readState = .readingHeader
				if (self.readBuffer[0..<10] != [82, 80, 67, 75, 73, 84, 0, 0, 0, 1]) {
					self.disconnect()
				}
				break
			case .readingHeader:
				// TODO: Check if the endianness is correct here.
				let header = Data(self.readBuffer[0..<4])
				let size = Int(header.withUnsafeBytes { $0.load(as: UInt32.self) }.bigEndian)
				self.readNeeds = size
				self.readState = .readingBody
				break
			case .readingBody:
				// This is terrible. This does a copy. We'll need to use a
				// native array to be able to get an ArraySlice, but that makes
				// the pointer part really ugly...
				let header = Data(self.readBuffer[0..<4])
				let size = Int(header.withUnsafeBytes { $0.load(as: UInt32.self) }.bigEndian)
				let message = readableMessage(slice: self.readBuffer[..<size])!
				self.readPos = 0
				self.readNeeds = 4
				self.readState = .readingHeader
				if case .failure(_) = self.gotMessage(msg: message) {
					self.disconnect()
				}
				break
			}
		}
	}

	fileprivate func flushWriteBuffer(stream: OutputStream) {
		// Just like the case with hasBytesAvailable, the buffer management
		// here is simple but dumb.
		if self.writeBuffer.count == 0 || self.connectionState != .connected {
			return
		}
		let cnt = self.writeBuffer.count;
		let written = self.writeBuffer.withUnsafeMutableBytes { ptr -> Int in
			let posptr = ptr.baseAddress!             // Get a raw pointer
				.assumingMemoryBound(to: UInt8.self)  // Make it a UInt8 pointer
			return stream.write(posptr, maxLength: cnt)
		}
		if written == -1 {
			// TODO: DIE!
		} else if written > 0 {
			self.writeBuffer.removeFirst(written)
		}
	}

	func stream(_ aStream: Stream, handle eventCode: Stream.Event) {
		switch eventCode {
		case Stream.Event.openCompleted:
			self.connectionState = .connected
			break
		case Stream.Event.hasBytesAvailable:
			self.hasBytesAvailable(stream: aStream as! InputStream)
			break
		case Stream.Event.endEncountered:
			self.disconnect()
			break
		case Stream.Event.errorOccurred:
			self.disconnect()
			break
		case Stream.Event.hasSpaceAvailable:
			self.flushWriteBuffer(stream: aStream as! OutputStream)
			break
		default:
			break
		}
	}

	fileprivate func gotMessage(msg: readableMessage) -> Result<Void, RPCError> {
		do {
			let mtraw = try msg.readVarUInt().get()
			guard let mt = RPCMessageType(rawValue: mtraw) else {
				return .failure(RPCError.protocolError)
			}
			switch mt {
			case RPCMessageType.methodCall:
				let callID = try msg.readVarUInt().get();
				let protocolID = try msg.readVarUInt().get();
				let server = self.servers[protocolID]
				if server == nil {
					return .failure(RPCError.protocolError)
				}
				let methodID = try msg.readVarUInt().get();
				let resp = try server!.handle(methodID: methodID, rmsg: msg).get();

				let wmsg = writableMessage()
				wmsg.writeVarUInt(value: RPCMessageType.methodReturn.rawValue)
				wmsg.writeVarUInt(value: callID)
				wmsg.writeVarUInt(value: resp.id())
				if case .failure(let error) = resp.encode(wmsg) {
					return .failure(error)
				}
				wmsg.finish()

				self.send(wmsg: wmsg)
				return .success(())
			case RPCMessageType.methodReturn:
				let callID = try msg.readVarUInt().get();
				let callSlot = self.findAndReleaseCallSlot(id: callID)
				if callSlot == nil {
					return .failure(RPCError.protocolError)
				}
				try callSlot!.callback(msg).get()
				return .success(())
			}
		} catch {
			self.disconnect()
			return .failure(RPCError.protocolError)
		}
	}
}

fileprivate enum protocolPingpongMethod: UInt64 {
	case SimpleTest = 1
	case ArrayTest = 2
}







// The PingpongServer protocol defines the pingpong protocol.
protocol PingpongServer {
	// The simplest of tests
	func SimpleTest(vinteger: Int64, vint64: Int64, vfloat: Float, vdouble: Double, vbool: Bool, vstring: String, vbytes: ArraySlice<UInt8>) -> Result<(Int64, Int64, Float, Double, Bool, String, ArraySlice<UInt8>), RPCError>

	// The simplest of tests, but with arrays
	func ArrayTest(vinteger: [Int64], vint64: [Int64], vfloat: [Float], vdouble: [Double], vbool: [Bool], vstring: [String], vbytes: [ArraySlice<UInt8>]) -> Result<([Int64], [Int64], [Float], [Double], [Bool], [String], [ArraySlice<UInt8>]), RPCError>

}

class RPCPingpongServer : RPCCallServer {
	private let impl: PingpongServer

	init(impl: PingpongServer) {
		self.impl = impl
	}

	func id() -> UInt64 {
		return 1
	}

	func handle(methodID: UInt64, rmsg: readableMessage) -> Result<RPCMessage, RPCError> {
		guard let method = protocolPingpongMethod(rawValue: methodID) else {
			return .failure(RPCError.protocolError)
		}

		switch method {
		case protocolPingpongMethod.SimpleTest:
			return self.handleSimpleTest(rmsg)
		case protocolPingpongMethod.ArrayTest:
			return self.handleArrayTest(rmsg)
		}
	}

	func handleSimpleTest(_ rmsg: readableMessage) -> Result<RPCMessage, RPCError> {
		// Get the message class for the call and decode into it
		let args: pingpongMethodSimpleTestCall
		switch pingpongMethodSimpleTestCall.decode(rmsg) {
		case .success(let v):
			args = v as! pingpongMethodSimpleTestCall
		case .failure(let error):
			return .failure(error)
		}
		// Call the user-provided implementation
		let res = self.impl.SimpleTest(vinteger: args.vinteger, vint64: args.vint64, vfloat: args.vfloat, vdouble: args.vdouble, vbool: args.vbool, vstring: args.vstring, vbytes: args.vbytes)

		// Unpack the return Result<tuple, Error>
		switch res {
		case .success(let res):
			// Construct the return message
			let retarg = pingpongMethodSimpleTestReturn(res)
			return .success(retarg)
		case .failure(let error):
			return .failure(error)
		}
	}

	func handleArrayTest(_ rmsg: readableMessage) -> Result<RPCMessage, RPCError> {
		// Get the message class for the call and decode into it
		let args: pingpongMethodArrayTestCall
		switch pingpongMethodArrayTestCall.decode(rmsg) {
		case .success(let v):
			args = v as! pingpongMethodArrayTestCall
		case .failure(let error):
			return .failure(error)
		}
		// Call the user-provided implementation
		let res = self.impl.ArrayTest(vinteger: args.vinteger, vint64: args.vint64, vfloat: args.vfloat, vdouble: args.vdouble, vbool: args.vbool, vstring: args.vstring, vbytes: args.vbytes)

		// Unpack the return Result<tuple, Error>
		switch res {
		case .success(let res):
			// Construct the return message
			let retarg = pingpongMethodArrayTestReturn(res)
			return .success(retarg)
		case .failure(let error):
			return .failure(error)
		}
	}
}

// The RPCPingpongClient type is a RPC client for the pingpong protocol.
class RPCPingpongClient {
	let conn: RPCConnection

	init(conn: RPCConnection) {
		self.conn = conn
	}

	func SimpleTest(vinteger: Int64, vint64: Int64, vfloat: Float, vdouble: Double, vbool: Bool, vstring: String, vbytes: ArraySlice<UInt8>, callback: @escaping (Result<(Int64, Int64, Float, Double, Bool, String, ArraySlice<UInt8>), Error>) -> ()) {

		// Names prefixed with __rpckit2 to avoid argument name collisions

		let __rpckit2_callID = self.conn.acquireCallSlot(callback: { (rmsg) -> Result<Void, RPCError> in
			guard let resultTypeID = try? rmsg.readVarUInt().get() else {
				callback(.failure(RPCError.protocolError))
				return .failure(RPCError.protocolError)
			}
			switch resultTypeID {
			case protocolPingpongMethod.SimpleTest.rawValue:
				switch pingpongMethodSimpleTestReturn.decode(rmsg) {
				case .success(let v):
					let ret = v as! pingpongMethodSimpleTestReturn
					callback(.success((ret.vinteger, ret.vint64, ret.vfloat, ret.vdouble, ret.vbool, ret.vstring, ret.vbytes)))
					return .success(())
				case .failure(let error):
					callback(.failure(error))
					return .failure(error)
				}
			case UInt64.max:
				switch rpcError.decode(rmsg) {
				case .success(let v):
					let ret = v as! rpcError
					callback(.failure(RPCError.applicationError(ret.error)))
					return .success(())
				case .failure(let error):
					callback(.failure(error))
					return .failure(error)
				}
			default:
				callback(.failure(RPCError.protocolError))
				return .failure(RPCError.protocolError)
			}
		})

		let __rpckit2_wmsg = writableMessage()

		// Write the protocol header
		__rpckit2_wmsg.writeVarUInt(value: RPCMessageType.methodCall.rawValue)
		__rpckit2_wmsg.writeVarUInt(value: __rpckit2_callID)
		__rpckit2_wmsg.writeVarUInt(value: 1)
		__rpckit2_wmsg.writeVarUInt(value: protocolPingpongMethod.SimpleTest.rawValue)

		let __rpckit2_callarg = pingpongMethodSimpleTestCall((vinteger, vint64, vfloat, vdouble, vbool, vstring, vbytes))
		if case .failure(let error) = __rpckit2_callarg.encode(__rpckit2_wmsg) {
				callback(.failure(RPCError.protocolError))
			callback(.failure(error))
			return ()
		}

		__rpckit2_wmsg.finish()

		self.conn.send(wmsg: __rpckit2_wmsg)
	}



	func ArrayTest(vinteger: [Int64], vint64: [Int64], vfloat: [Float], vdouble: [Double], vbool: [Bool], vstring: [String], vbytes: [ArraySlice<UInt8>], callback: @escaping (Result<([Int64], [Int64], [Float], [Double], [Bool], [String], [ArraySlice<UInt8>]), Error>) -> ()) {

		// Names prefixed with __rpckit2 to avoid argument name collisions

		let __rpckit2_callID = self.conn.acquireCallSlot(callback: { (rmsg) -> Result<Void, RPCError> in
			guard let resultTypeID = try? rmsg.readVarUInt().get() else {
				callback(.failure(RPCError.protocolError))
				return .failure(RPCError.protocolError)
			}
			switch resultTypeID {
			case protocolPingpongMethod.ArrayTest.rawValue:
				switch pingpongMethodArrayTestReturn.decode(rmsg) {
				case .success(let v):
					let ret = v as! pingpongMethodArrayTestReturn
					callback(.success((ret.vinteger, ret.vint64, ret.vfloat, ret.vdouble, ret.vbool, ret.vstring, ret.vbytes)))
					return .success(())
				case .failure(let error):
					callback(.failure(error))
					return .failure(error)
				}
			case UInt64.max:
				switch rpcError.decode(rmsg) {
				case .success(let v):
					let ret = v as! rpcError
					callback(.failure(RPCError.applicationError(ret.error)))
					return .success(())
				case .failure(let error):
					callback(.failure(error))
					return .failure(error)
				}
			default:
				callback(.failure(RPCError.protocolError))
				return .failure(RPCError.protocolError)
			}
		})

		let __rpckit2_wmsg = writableMessage()

		// Write the protocol header
		__rpckit2_wmsg.writeVarUInt(value: RPCMessageType.methodCall.rawValue)
		__rpckit2_wmsg.writeVarUInt(value: __rpckit2_callID)
		__rpckit2_wmsg.writeVarUInt(value: 1)
		__rpckit2_wmsg.writeVarUInt(value: protocolPingpongMethod.ArrayTest.rawValue)

		let __rpckit2_callarg = pingpongMethodArrayTestCall((vinteger, vint64, vfloat, vdouble, vbool, vstring, vbytes))
		if case .failure(let error) = __rpckit2_callarg.encode(__rpckit2_wmsg) {
				callback(.failure(RPCError.protocolError))
			callback(.failure(error))
			return ()
		}

		__rpckit2_wmsg.finish()

		self.conn.send(wmsg: __rpckit2_wmsg)
	}


}


class rpcError : RPCMessage {
	fileprivate var error: String
	private var errorType: RPCErrorType

	init() {
		self.error = ""
		self.errorType = .generic
	}

	init(_ args: (String, RPCErrorType)) {
		(self.error, self.errorType) = args
	}

	func id() -> UInt64 {
		return UInt64.max
	}

	func encode(_ wmsg: writableMessage) -> Result<(), RPCError> {
		wmsg.writePBUInt64(fieldNumber: 1, value: self.errorType.rawValue)
		wmsg.writePBString(fieldNumber: 2, value: self.error)
		return .success(())
	}

	static func decode(_ rmsg: readableMessage) -> Result<RPCMessage, RPCError> {
		var error: String = ""
		var errorType: RPCErrorType = .generic
		do {
			while true {
				let tag = try rmsg.readVarUInt().get()
				switch tag {
				case (1 << 3 | RPCWireType.varInt.rawValue):
					guard let v = RPCErrorType(rawValue: try! rmsg.readUInt64().get()) else {
						return .failure(.protocolError)
					}
					errorType = v
					break
				case (2 << 3 | RPCWireType.fixed64Bit.rawValue):
					error = try rmsg.readString().get()
					break
				default:
					break
				}
			}
		} catch RPCError.eof {
		} catch let error as RPCError {
			return .failure(error)
		} catch {
			return .failure(RPCError.protocolError)
		}
		return .success(rpcError((error, errorType)))
	}
}
































































































fileprivate class pingpongMethodSimpleTestCall : RPCMessage {
	var vinteger: Int64
	var vint64: Int64
	var vfloat: Float
	var vdouble: Double
	var vbool: Bool
	var vstring: String
	var vbytes: ArraySlice<UInt8>

	init() {
		self.vinteger = 0
		self.vint64 = 0
		self.vfloat = 0.0
		self.vdouble = 0.0
		self.vbool = false
		self.vstring = ""
		self.vbytes = []
	}

	init(_ args: (Int64, Int64, Float, Double, Bool, String, ArraySlice<UInt8>)) {
		(self.vinteger, self.vint64, self.vfloat, self.vdouble, self.vbool, self.vstring, self.vbytes) = args
	}

	func id() -> UInt64 {
		return protocolPingpongMethod.SimpleTest.rawValue
	}

	func encode(_ __rpckit2_wmsg: writableMessage) -> Result<(), RPCError> {
		// Write the auto-generated message
__rpckit2_wmsg.writePBInt(fieldNumber: 1, value: self.vinteger)
__rpckit2_wmsg.writePBInt64(fieldNumber: 2, value: self.vint64)
__rpckit2_wmsg.writePBFloat(fieldNumber: 3, value: self.vfloat)
__rpckit2_wmsg.writePBDouble(fieldNumber: 4, value: self.vdouble)
__rpckit2_wmsg.writePBBool(fieldNumber: 5, value: self.vbool)
__rpckit2_wmsg.writePBString(fieldNumber: 6, value: self.vstring)
__rpckit2_wmsg.writePBBytes(fieldNumber: 7, value: self.vbytes)
		return .success(())
	}

	static func decode(_ __rpckit2_rmsg: readableMessage) -> Result<RPCMessage, RPCError> {
		var args: (vinteger: Int64, vint64: Int64, vfloat: Float, vdouble: Double, vbool: Bool, vstring: String, vbytes: ArraySlice<UInt8>) = (0, 0, 0.0, 0.0, false, "", [])
		do {
			while true {
				let tag = try __rpckit2_rmsg.readVarUInt().get()
				switch tag {
				case (1 << 3 | RPCWireType.varInt.rawValue):
args.vinteger = try __rpckit2_rmsg.readInt().get()
					break
				case (2 << 3 | RPCWireType.fixed64Bit.rawValue):
args.vint64 = try __rpckit2_rmsg.readInt64().get()
					break
				case (3 << 3 | RPCWireType.fixed32Bit.rawValue):
args.vfloat = try __rpckit2_rmsg.readFloat().get()
					break
				case (4 << 3 | RPCWireType.fixed64Bit.rawValue):
args.vdouble = try __rpckit2_rmsg.readDouble().get()
					break
				case (5 << 3 | RPCWireType.varInt.rawValue):
args.vbool = try __rpckit2_rmsg.readBool().get()
					break
				case (6 << 3 | RPCWireType.lengthDelimited.rawValue):
args.vstring = try __rpckit2_rmsg.readString().get()
					break
				case (7 << 3 | RPCWireType.lengthDelimited.rawValue):
args.vbytes = try __rpckit2_rmsg.readBytes().get()
					break
				default:
					return .failure(RPCError.protocolError)
				}
			}
		} catch RPCError.eof {
			// Not a problem
		} catch let error as RPCError {
			return .failure(error)
		} catch {
			return .failure(RPCError.protocolError)
		}
		return .success(pingpongMethodSimpleTestCall(args))
	}
}
fileprivate class pingpongMethodSimpleTestReturn : RPCMessage {
	var vinteger: Int64
	var vint64: Int64
	var vfloat: Float
	var vdouble: Double
	var vbool: Bool
	var vstring: String
	var vbytes: ArraySlice<UInt8>

	init() {
		self.vinteger = 0
		self.vint64 = 0
		self.vfloat = 0.0
		self.vdouble = 0.0
		self.vbool = false
		self.vstring = ""
		self.vbytes = []
	}

	init(_ args: (Int64, Int64, Float, Double, Bool, String, ArraySlice<UInt8>)) {
		(self.vinteger, self.vint64, self.vfloat, self.vdouble, self.vbool, self.vstring, self.vbytes) = args
	}

	func id() -> UInt64 {
		return protocolPingpongMethod.SimpleTest.rawValue
	}

	func encode(_ __rpckit2_wmsg: writableMessage) -> Result<(), RPCError> {
		// Write the auto-generated message
__rpckit2_wmsg.writePBInt(fieldNumber: 1, value: self.vinteger)
__rpckit2_wmsg.writePBInt64(fieldNumber: 2, value: self.vint64)
__rpckit2_wmsg.writePBFloat(fieldNumber: 3, value: self.vfloat)
__rpckit2_wmsg.writePBDouble(fieldNumber: 4, value: self.vdouble)
__rpckit2_wmsg.writePBBool(fieldNumber: 5, value: self.vbool)
__rpckit2_wmsg.writePBString(fieldNumber: 6, value: self.vstring)
__rpckit2_wmsg.writePBBytes(fieldNumber: 7, value: self.vbytes)
		return .success(())
	}

	static func decode(_ __rpckit2_rmsg: readableMessage) -> Result<RPCMessage, RPCError> {
		var args: (vinteger: Int64, vint64: Int64, vfloat: Float, vdouble: Double, vbool: Bool, vstring: String, vbytes: ArraySlice<UInt8>) = (0, 0, 0.0, 0.0, false, "", [])
		do {
			while true {
				let tag = try __rpckit2_rmsg.readVarUInt().get()
				switch tag {
				case (1 << 3 | RPCWireType.varInt.rawValue):
args.vinteger = try __rpckit2_rmsg.readInt().get()
					break
				case (2 << 3 | RPCWireType.fixed64Bit.rawValue):
args.vint64 = try __rpckit2_rmsg.readInt64().get()
					break
				case (3 << 3 | RPCWireType.fixed32Bit.rawValue):
args.vfloat = try __rpckit2_rmsg.readFloat().get()
					break
				case (4 << 3 | RPCWireType.fixed64Bit.rawValue):
args.vdouble = try __rpckit2_rmsg.readDouble().get()
					break
				case (5 << 3 | RPCWireType.varInt.rawValue):
args.vbool = try __rpckit2_rmsg.readBool().get()
					break
				case (6 << 3 | RPCWireType.lengthDelimited.rawValue):
args.vstring = try __rpckit2_rmsg.readString().get()
					break
				case (7 << 3 | RPCWireType.lengthDelimited.rawValue):
args.vbytes = try __rpckit2_rmsg.readBytes().get()
					break
				default:
					return .failure(RPCError.protocolError)
				}
			}
		} catch RPCError.eof {
			// Not a problem
		} catch let error as RPCError {
			return .failure(error)
		} catch {
			return .failure(RPCError.protocolError)
		}
		return .success(pingpongMethodSimpleTestReturn(args))
	}
}
	
fileprivate class pingpongMethodArrayTestCall : RPCMessage {
	var vinteger: [Int64]
	var vint64: [Int64]
	var vfloat: [Float]
	var vdouble: [Double]
	var vbool: [Bool]
	var vstring: [String]
	var vbytes: [ArraySlice<UInt8>]

	init() {
		self.vinteger = []
		self.vint64 = []
		self.vfloat = []
		self.vdouble = []
		self.vbool = []
		self.vstring = []
		self.vbytes = []
	}

	init(_ args: ([Int64], [Int64], [Float], [Double], [Bool], [String], [ArraySlice<UInt8>])) {
		(self.vinteger, self.vint64, self.vfloat, self.vdouble, self.vbool, self.vstring, self.vbytes) = args
	}

	func id() -> UInt64 {
		return protocolPingpongMethod.ArrayTest.rawValue
	}

	func encode(_ __rpckit2_wmsg: writableMessage) -> Result<(), RPCError> {
		// Write the auto-generated message
for v in self.vinteger {
	__rpckit2_wmsg.writePBInt(fieldNumber: 1, value: v)
}
for v in self.vint64 {
	__rpckit2_wmsg.writePBInt64(fieldNumber: 2, value: v)
}
for v in self.vfloat {
	__rpckit2_wmsg.writePBFloat(fieldNumber: 3, value: v)
}
for v in self.vdouble {
	__rpckit2_wmsg.writePBDouble(fieldNumber: 4, value: v)
}
for v in self.vbool {
	__rpckit2_wmsg.writePBBool(fieldNumber: 5, value: v)
}
for v in self.vstring {
	__rpckit2_wmsg.writePBString(fieldNumber: 6, value: v)
}
for v in self.vbytes {
	__rpckit2_wmsg.writePBBytes(fieldNumber: 7, value: v)
}
		return .success(())
	}

	static func decode(_ __rpckit2_rmsg: readableMessage) -> Result<RPCMessage, RPCError> {
		var args: (vinteger: [Int64], vint64: [Int64], vfloat: [Float], vdouble: [Double], vbool: [Bool], vstring: [String], vbytes: [ArraySlice<UInt8>]) = ([], [], [], [], [], [], [])
		do {
			while true {
				let tag = try __rpckit2_rmsg.readVarUInt().get()
				switch tag {
				case (1 << 3 | RPCWireType.varInt.rawValue):
let v: Int64
v = try __rpckit2_rmsg.readInt().get()
args.vinteger.append(v)
					break
				case (2 << 3 | RPCWireType.fixed64Bit.rawValue):
let v: Int64
v = try __rpckit2_rmsg.readInt64().get()
args.vint64.append(v)
					break
				case (3 << 3 | RPCWireType.fixed32Bit.rawValue):
let v: Float
v = try __rpckit2_rmsg.readFloat().get()
args.vfloat.append(v)
					break
				case (4 << 3 | RPCWireType.fixed64Bit.rawValue):
let v: Double
v = try __rpckit2_rmsg.readDouble().get()
args.vdouble.append(v)
					break
				case (5 << 3 | RPCWireType.varInt.rawValue):
let v: Bool
v = try __rpckit2_rmsg.readBool().get()
args.vbool.append(v)
					break
				case (6 << 3 | RPCWireType.lengthDelimited.rawValue):
let v: String
v = try __rpckit2_rmsg.readString().get()
args.vstring.append(v)
					break
				case (7 << 3 | RPCWireType.lengthDelimited.rawValue):
let v: ArraySlice<UInt8>
v = try __rpckit2_rmsg.readBytes().get()
args.vbytes.append(v)
					break
				default:
					return .failure(RPCError.protocolError)
				}
			}
		} catch RPCError.eof {
			// Not a problem
		} catch let error as RPCError {
			return .failure(error)
		} catch {
			return .failure(RPCError.protocolError)
		}
		return .success(pingpongMethodArrayTestCall(args))
	}
}
fileprivate class pingpongMethodArrayTestReturn : RPCMessage {
	var vinteger: [Int64]
	var vint64: [Int64]
	var vfloat: [Float]
	var vdouble: [Double]
	var vbool: [Bool]
	var vstring: [String]
	var vbytes: [ArraySlice<UInt8>]

	init() {
		self.vinteger = []
		self.vint64 = []
		self.vfloat = []
		self.vdouble = []
		self.vbool = []
		self.vstring = []
		self.vbytes = []
	}

	init(_ args: ([Int64], [Int64], [Float], [Double], [Bool], [String], [ArraySlice<UInt8>])) {
		(self.vinteger, self.vint64, self.vfloat, self.vdouble, self.vbool, self.vstring, self.vbytes) = args
	}

	func id() -> UInt64 {
		return protocolPingpongMethod.ArrayTest.rawValue
	}

	func encode(_ __rpckit2_wmsg: writableMessage) -> Result<(), RPCError> {
		// Write the auto-generated message
for v in self.vinteger {
	__rpckit2_wmsg.writePBInt(fieldNumber: 1, value: v)
}
for v in self.vint64 {
	__rpckit2_wmsg.writePBInt64(fieldNumber: 2, value: v)
}
for v in self.vfloat {
	__rpckit2_wmsg.writePBFloat(fieldNumber: 3, value: v)
}
for v in self.vdouble {
	__rpckit2_wmsg.writePBDouble(fieldNumber: 4, value: v)
}
for v in self.vbool {
	__rpckit2_wmsg.writePBBool(fieldNumber: 5, value: v)
}
for v in self.vstring {
	__rpckit2_wmsg.writePBString(fieldNumber: 6, value: v)
}
for v in self.vbytes {
	__rpckit2_wmsg.writePBBytes(fieldNumber: 7, value: v)
}
		return .success(())
	}

	static func decode(_ __rpckit2_rmsg: readableMessage) -> Result<RPCMessage, RPCError> {
		var args: (vinteger: [Int64], vint64: [Int64], vfloat: [Float], vdouble: [Double], vbool: [Bool], vstring: [String], vbytes: [ArraySlice<UInt8>]) = ([], [], [], [], [], [], [])
		do {
			while true {
				let tag = try __rpckit2_rmsg.readVarUInt().get()
				switch tag {
				case (1 << 3 | RPCWireType.varInt.rawValue):
let v: Int64
v = try __rpckit2_rmsg.readInt().get()
args.vinteger.append(v)
					break
				case (2 << 3 | RPCWireType.fixed64Bit.rawValue):
let v: Int64
v = try __rpckit2_rmsg.readInt64().get()
args.vint64.append(v)
					break
				case (3 << 3 | RPCWireType.fixed32Bit.rawValue):
let v: Float
v = try __rpckit2_rmsg.readFloat().get()
args.vfloat.append(v)
					break
				case (4 << 3 | RPCWireType.fixed64Bit.rawValue):
let v: Double
v = try __rpckit2_rmsg.readDouble().get()
args.vdouble.append(v)
					break
				case (5 << 3 | RPCWireType.varInt.rawValue):
let v: Bool
v = try __rpckit2_rmsg.readBool().get()
args.vbool.append(v)
					break
				case (6 << 3 | RPCWireType.lengthDelimited.rawValue):
let v: String
v = try __rpckit2_rmsg.readString().get()
args.vstring.append(v)
					break
				case (7 << 3 | RPCWireType.lengthDelimited.rawValue):
let v: ArraySlice<UInt8>
v = try __rpckit2_rmsg.readBytes().get()
args.vbytes.append(v)
					break
				default:
					return .failure(RPCError.protocolError)
				}
			}
		} catch RPCError.eof {
			// Not a problem
		} catch let error as RPCError {
			return .failure(error)
		} catch {
			return .failure(RPCError.protocolError)
		}
		return .success(pingpongMethodArrayTestReturn(args))
	}
}
	

