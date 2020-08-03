import Foundation





enum rpckitError: Error {
	case eof
	case varIntOverflow
	case unknownTag
	case encodingError
	case protocolError
}

enum errorType: UInt64 {
	case generic = 0
	case timeout = 1
	case proto = 2
	case application = 3
	case connection = 4
	case shutdown = 5
}

private enum wireType: UInt64 {
	case varInt = 0
	case fixed64Bit = 1
	case lengthDelimited = 2
	case fixed32Bit = 5
}

private enum messageType: UInt64 {
	case methodCall = 1
	case methodReturn = 2
}

private class readableMessage {
	var pos: Int = 0
	var len: Int = 0
	var buf: ArraySlice<UInt8>

	init?(slice: ArraySlice<UInt8>) {
		if (slice.count <= 4) {
			return nil
		}
		self.buf = slice
		self.pos = 4
		self.len = Int( Data(slice[..<4]).withUnsafeBytes { $0.load(as: UInt32.self) }.bigEndian )
	}

	init?(embeddedSlice: ArraySlice<UInt8>) {
		self.buf = embeddedSlice
		self.len = buf.count
	}

	func readBytes() -> Result<ArraySlice<UInt8>, rpckitError> {
		var len: Int
		switch self.readVarUInt() {
		case .success(let val):
			len = Int(val)
		case .failure(let error):
			return .failure(error)
		}

		if self.pos + len > self.len {
			return .failure(rpckitError.eof)
		}
		let slice = self.buf[self.pos ..< self.pos + len]
		self.pos += len
		return .success(slice)
	}

	func readString() -> Result<String, rpckitError> {
		switch self.readBytes() {
		case .success(let val):
			if let string = String(data: Data(val), encoding: .utf8) {
				return .success(string)
			} else {
				return .failure(rpckitError.encodingError)
			}
		case .failure(let error):
			return .failure(error)
		}
	}

	func readVarUInt() -> Result<UInt64, rpckitError> {
		var x: UInt64 = 0
		var s: UInt = 0
		for index in 0..<self.len {
			if self.pos+1 > self.len {
				return .failure(rpckitError.eof)
			}
			let b = self.buf[self.pos]
			self.pos += 1
			if b < 0x80 {
				if index > 9 || (index == 9 && b > 1) {
					return .failure(rpckitError.varIntOverflow)
				}
				return .success(x | UInt64(b) << s)
			}
			x |= UInt64(b & 0x7F) << s
			s += 7
		}
		return .failure(rpckitError.varIntOverflow)
	}

	func readVarInt() -> Result<Int64, rpckitError> {
		switch self.readVarUInt() {
		case .success(let val):
			return .success(Int64(val))
		case .failure(let error):
			return .failure(error)
		}
	}

	func readInt() -> Result<Int64, rpckitError> {
		return self.readVarInt()
	}

	func readUInt32() -> Result<UInt32, rpckitError> {
		if self.pos + 4 > self.len {
			return .failure(rpckitError.eof)
		}
		let val = Data(self.buf[self.pos..<self.pos+4]).withUnsafeBytes { $0.load(as: UInt32.self) }
		self.pos += 4
		return .success(val)
	}

	func readUInt64() -> Result<UInt64, rpckitError> {
		if self.pos + 8 > self.len {
			return .failure(rpckitError.eof)
		}
		let val = Data(self.buf[self.pos..<self.pos+8]).withUnsafeBytes { $0.load(as: UInt64.self) }
		self.pos += 8
		return .success(val)
	}

	func readInt32() -> Result<Int32, rpckitError> {
		switch self.readUInt32() {
		case .success(let val):
			return .success(Int32(val))
		case .failure(let error):
			return .failure(error)
		}
	}

	func readInt64() -> Result<Int64, rpckitError> {
		switch self.readUInt64() {
		case .success(let val):
			return .success(Int64(val))
		case .failure(let error):
			return .failure(error)
		}
	}

	func readBool() -> Result<Bool, rpckitError> {
		switch self.readVarUInt() {
		case .success(let val):
			return .success(val == 1)
		case .failure(let error):
			return .failure(error)
		}
	}

	func readFloat() -> Result<Float, rpckitError> {
		if self.pos + 4 > self.len {
			return .failure(rpckitError.eof)
		}
		let val = Data(self.buf[self.pos..<self.pos+4]).withUnsafeBytes { $0.load(as: Float.self) }
		self.pos += 4
		return .success(val)
	}

	func readDouble() -> Result<Double, rpckitError> {
		if self.pos + 8 > self.len {
			return .failure(rpckitError.eof)
		}
		let val = Data(self.buf[self.pos..<self.pos+8]).withUnsafeBytes { $0.load(as: Double.self) }
		self.pos += 8
		return .success(val)
	}

	func readPBSkip(tag: UInt64) -> Result<Void, rpckitError> {
		guard let t = wireType(rawValue: tag & ((1 << 3) - 1)) else {
			return .failure(rpckitError.unknownTag)
		}
		switch t {
		case wireType.varInt:
			switch self.readVarUInt() {
			case .success(_):
				return .success(())
			case .failure(let error):
				return .failure(error)
			}
		case wireType.fixed64Bit:
			switch self.readInt64() {
			case .success(_):
				return .success(())
			case .failure(let error):
				return .failure(error)
			}
		case wireType.lengthDelimited:
			switch self.readBytes() {
			case .success(_):
				return .success(())
			case .failure(let error):
				return .failure(error)
			}
		case wireType.fixed32Bit:
			switch self.readInt32() {
			case .success(_):
				return .success(())
			case .failure(let error):
				return .failure(error)
			}
		}
	}
}

private class writableMessage {
	var buf: [UInt8] = []
	var embedded: Bool = false

	init() {
		self.buf.reserveCapacity(1024)
		for _ in 0..<4 {
			self.buf.append(0)
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

	func writePBTag(fieldNumber: UInt64, wt: wireType) {
		self.writeVarUInt(value: (fieldNumber << 3) | wt.rawValue)
	}

	func writePBString(fieldNumber: UInt64, value: String) {
		if value.count > 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.lengthDelimited)
			self.writeString(str: value)
		}
	}

	func writePBBytes(fieldNumber: UInt64, value: ArraySlice<UInt8>) {
		if value.count > 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.lengthDelimited)
			self.writeBytes(bytes: value)
		}
	}

	func writePBBool(fieldNumber: UInt64, value: Bool) {
		if value {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.varInt)
			self.writeVarUInt(value: 1)
		}
	}

	func writePBVarUInt(fieldNumber: UInt64, value: UInt64) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.varInt)
			self.writeVarUInt(value: value)
		}
	}

	func writePBVarInt(fieldNumber: UInt64, value: Int64) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.varInt)
			self.writeVarInt(value: value)
		}
	}

	func writePBInt(fieldNumber: UInt64, value: Int64) {
		return self.writePBVarInt(fieldNumber: fieldNumber, value: value)
	}

	func writePBInt64(fieldNumber: UInt64, value: Int64) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.fixed64Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBUInt64(fieldNumber: UInt64, value: UInt64) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.fixed64Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBInt32(fieldNumber: UInt64, value: Int32) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.fixed32Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBUInt64(fieldNumber: UInt64, value: UInt32) {
		if value != 0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.fixed32Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBFloat(fieldNumber: UInt64, value: Float) {
		if value != 0.0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.fixed32Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBDouble(fieldNumber: UInt64, value: Double) {
		if value != 0.0 {
			self.writePBTag(fieldNumber: fieldNumber, wt: wireType.fixed64Bit)
			self.writeSimple(value: value)
		}
	}

	func writePBMessage(fieldNumber: UInt64, msg: writableMessage) {
		self.writeBytes(bytes: msg.buf[...])
	}
}

private protocol rpcMessage {
	func rpcEncode(msg: writableMessage) -> Result<Void, rpckitError>
	func rpcDecode(msg: writableMessage) -> Result<Void, rpckitError>
	func rpcID() -> UInt64
}

private protocol rpcCallable {
	func call(server: rpcCallServer) -> rpcMessage
}

private protocol rpcCallServer {
	func handle(methodID: UInt64, msg: readableMessage) -> Result<rpcCallable, rpckitError>;
}

private class callSlot {
	let callback: (readableMessage) -> Result<Void, rpckitError>

	init(callback: @escaping (readableMessage) -> Result<Void, rpckitError>) {
		self.callback = callback
	}
}

private enum rpcConnectionState {
	case disconnected
	case connecting
	case connected
}

private enum rpcReaderState {
	case readingPreamble
	case readingHeader
	case readingBody
}

public class RPCConnection: NSObject, StreamDelegate {
	// Protocol state
	private var nextID: UInt64 = 0
	private var slots: [UInt64: callSlot] = [:]

	// Immutable paramters
	private let host: String
	private let port: Int
	private let tls: Bool
	private let servers: [UInt64 : rpcCallServer]

	// Connection
	private var connectionState: rpcConnectionState
	private var inputStream: InputStream!
	private var outputStream: OutputStream!

	private var readState: rpcReaderState
	private var readNeeds: Int
	private var readPos: Int
	private var readBuffer: [UInt8]

	private var writeBuffer: [UInt8]

	private init(host: String, port: Int, tls: Bool, servers: [UInt64 : rpcCallServer]) {
		self.host = host
		self.port = port
		self.tls = tls
		self.servers = servers
		self.connectionState = rpcConnectionState.disconnected

		self.readState = rpcReaderState.readingPreamble
		self.readNeeds = 10
		self.readPos = 0
		self.readBuffer = Array(repeating: 0, count: 1024)
		self.writeBuffer = []
	}


	convenience init(host: String, port: Int, tls: Bool) {
		self.init(host: host, port: port, tls: tls, servers: [:])
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

	fileprivate func acquireCallSlot(callback: @escaping (readableMessage) -> Result<Void, rpckitError>) -> UInt64 {
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

	public func stream(_ aStream: Stream, handle eventCode: Stream.Event) {
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

	fileprivate func gotMessage(msg: readableMessage) -> Result<Void, rpckitError> {
		do {
			let mtraw = try msg.readVarUInt().get()
			guard let mt = messageType(rawValue: mtraw) else {
				return .failure(rpckitError.protocolError)
			}
			switch mt {
			case messageType.methodCall:
				let callID = try msg.readVarUInt().get();
				let protocolID = try msg.readVarUInt().get();
				let server = self.servers[protocolID]
				if server == nil {
					return .failure(rpckitError.protocolError)
				}
				let methodID = try msg.readVarUInt().get();
				let callable = try server!.handle(methodID: methodID, msg: msg).get();

				let resp = callable.call(server: server!)
				let wmsg = writableMessage()
				wmsg.writeVarUInt(value: messageType.methodReturn.rawValue)
				wmsg.writeVarUInt(value: callID)
				wmsg.writeVarUInt(value: resp.rpcID())
				if case .failure(let error) = resp.rpcEncode(msg: wmsg) {
					return .failure(error)
				}
				wmsg.finish()

				self.send(wmsg: wmsg)
				return .success(())
			case messageType.methodReturn:
				let callID = try msg.readVarUInt().get();
				let callSlot = self.findAndReleaseCallSlot(id: callID)
				if callSlot == nil {
					return .failure(rpckitError.protocolError)
				}
				try callSlot!.callback(msg).get()
				break
			}
		} catch {
			self.disconnect()
			return .failure(rpckitError.protocolError)
		}
		return .failure(rpckitError.eof)
	}
}







private enum protocolPingpongMethod: UInt64 {
	case SimpleTest = 1
	
	case ArrayTest = 2
	
}





























// The RPCPingpongClient type is a RPC client for the pingpong protocol.
public class RPCPingpongClient {
	let conn: RPCConnection

	init(conn: RPCConnection) {
		self.conn = conn
	}

	public func SimpleTest(vinteger: Int64, vint64: Int64, vfloat: Float, vdouble: Double, vbool: Bool, vstring: String, vbytes: ArraySlice<UInt8>, callback: @escaping (Result<(Int64, Int64, Float, Double, Bool, String, ArraySlice<UInt8>), Error>) -> ()) -> Result<Void, Error> {

		// Names prefixed with __rpckit2 to avoid argument name collisions

		let __rpckit2_callID = self.conn.acquireCallSlot(callback: { (__rpckit2_rmsg) -> Result<Void, rpckitError> in
			var vinteger: Int64 = 0
			var vint64: Int64 = 0
			var vfloat: Float = 0.0
			var vdouble: Double = 0.0
			var vbool: Bool = false
			var vstring: String = ""
			var vbytes: ArraySlice<UInt8> = []
			
			do {
				while true {
					let tag = try __rpckit2_rmsg.readVarUInt().get()
					switch tag {
					case (1 << 3 | wireType.varInt.rawValue):
vinteger = try __rpckit2_rmsg.readInt().get()
						break
					case (2 << 3 | wireType.fixed64Bit.rawValue):
vint64 = try __rpckit2_rmsg.readInt64().get()
						break
					case (3 << 3 | wireType.fixed32Bit.rawValue):
vfloat = try __rpckit2_rmsg.readFloat().get()
						break
					case (4 << 3 | wireType.fixed64Bit.rawValue):
vdouble = try __rpckit2_rmsg.readDouble().get()
						break
					case (5 << 3 | wireType.varInt.rawValue):
vbool = try __rpckit2_rmsg.readBool().get()
						break
					case (6 << 3 | wireType.lengthDelimited.rawValue):
vstring = try __rpckit2_rmsg.readString().get()
						break
					case (7 << 3 | wireType.lengthDelimited.rawValue):
vbytes = try __rpckit2_rmsg.readBytes().get()
						break
					default:
						break
					}
				}
			} catch  {
			}

			callback(.success((vinteger, vint64, vfloat, vdouble, vbool, vstring, vbytes)))
			return .success(())
		})

		let __rpckit2_wmsg = writableMessage()

		// Write the protocol header
		__rpckit2_wmsg.writeVarUInt(value: messageType.methodCall.rawValue)
		__rpckit2_wmsg.writeVarUInt(value: __rpckit2_callID)
		__rpckit2_wmsg.writeVarUInt(value: 1)
		__rpckit2_wmsg.writeVarUInt(value: protocolPingpongMethod.SimpleTest.rawValue)

		// Write the auto-generated message
__rpckit2_wmsg.writePBInt(fieldNumber: 1, value: vinteger)
__rpckit2_wmsg.writePBInt64(fieldNumber: 2, value: vint64)
__rpckit2_wmsg.writePBFloat(fieldNumber: 3, value: vfloat)
__rpckit2_wmsg.writePBDouble(fieldNumber: 4, value: vdouble)
__rpckit2_wmsg.writePBBool(fieldNumber: 5, value: vbool)
__rpckit2_wmsg.writePBString(fieldNumber: 6, value: vstring)
__rpckit2_wmsg.writePBBytes(fieldNumber: 7, value: vbytes)
		__rpckit2_wmsg.finish()

		self.conn.send(wmsg: __rpckit2_wmsg)
		return .success(())
	}

	

	public func ArrayTest(vinteger: [Int64], vint64: [Int64], vfloat: [Float], vdouble: [Double], vbool: [Bool], vstring: [String], vbytes: [ArraySlice<UInt8>], callback: @escaping (Result<([Int64], [Int64], [Float], [Double], [Bool], [String], [ArraySlice<UInt8>]), Error>) -> ()) -> Result<Void, Error> {

		// Names prefixed with __rpckit2 to avoid argument name collisions

		let __rpckit2_callID = self.conn.acquireCallSlot(callback: { (__rpckit2_rmsg) -> Result<Void, rpckitError> in
			var vinteger: [Int64] = []
			var vint64: [Int64] = []
			var vfloat: [Float] = []
			var vdouble: [Double] = []
			var vbool: [Bool] = []
			var vstring: [String] = []
			var vbytes: [ArraySlice<UInt8>] = []
			
			do {
				while true {
					let tag = try __rpckit2_rmsg.readVarUInt().get()
					switch tag {
					case (1 << 3 | wireType.varInt.rawValue):
let v: Int64
v = try __rpckit2_rmsg.readInt().get()
vinteger.append(v)
						break
					case (2 << 3 | wireType.fixed64Bit.rawValue):
let v: Int64
v = try __rpckit2_rmsg.readInt64().get()
vint64.append(v)
						break
					case (3 << 3 | wireType.fixed32Bit.rawValue):
let v: Float
v = try __rpckit2_rmsg.readFloat().get()
vfloat.append(v)
						break
					case (4 << 3 | wireType.fixed64Bit.rawValue):
let v: Double
v = try __rpckit2_rmsg.readDouble().get()
vdouble.append(v)
						break
					case (5 << 3 | wireType.varInt.rawValue):
let v: Bool
v = try __rpckit2_rmsg.readBool().get()
vbool.append(v)
						break
					case (6 << 3 | wireType.lengthDelimited.rawValue):
let v: String
v = try __rpckit2_rmsg.readString().get()
vstring.append(v)
						break
					case (7 << 3 | wireType.lengthDelimited.rawValue):
let v: ArraySlice<UInt8>
v = try __rpckit2_rmsg.readBytes().get()
vbytes.append(v)
						break
					default:
						break
					}
				}
			} catch  {
			}

			callback(.success((vinteger, vint64, vfloat, vdouble, vbool, vstring, vbytes)))
			return .success(())
		})

		let __rpckit2_wmsg = writableMessage()

		// Write the protocol header
		__rpckit2_wmsg.writeVarUInt(value: messageType.methodCall.rawValue)
		__rpckit2_wmsg.writeVarUInt(value: __rpckit2_callID)
		__rpckit2_wmsg.writeVarUInt(value: 1)
		__rpckit2_wmsg.writeVarUInt(value: protocolPingpongMethod.ArrayTest.rawValue)

		// Write the auto-generated message
for v in vinteger {
	__rpckit2_wmsg.writePBInt(fieldNumber: 1, value: v)
}
for v in vint64 {
	__rpckit2_wmsg.writePBInt64(fieldNumber: 2, value: v)
}
for v in vfloat {
	__rpckit2_wmsg.writePBFloat(fieldNumber: 3, value: v)
}
for v in vdouble {
	__rpckit2_wmsg.writePBDouble(fieldNumber: 4, value: v)
}
for v in vbool {
	__rpckit2_wmsg.writePBBool(fieldNumber: 5, value: v)
}
for v in vstring {
	__rpckit2_wmsg.writePBString(fieldNumber: 6, value: v)
}
for v in vbytes {
	__rpckit2_wmsg.writePBBytes(fieldNumber: 7, value: v)
}
		__rpckit2_wmsg.finish()

		self.conn.send(wmsg: __rpckit2_wmsg)
		return .success(())
	}

	
}


