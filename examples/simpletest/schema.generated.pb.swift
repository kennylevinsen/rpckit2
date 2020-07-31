import Foundation





enum rpckitError: Error {
	case eof
	case varintOverflow
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

enum wireType: UInt64 {
	case varInt = 0
	case fixed64Bit = 1
	case lengthDelimited = 2
	case fixed32Bit = 5
}

enum messageType: UInt64 {
	case methodCall = 1
	case methodReturn = 2
}

class readableMessage {
	var pos: Int = 0
	var len: Int = 0
	var buf: ArraySlice<UInt8>

	init?(slice: ArraySlice<UInt8>) {
		self.buf = slice
		if (buf.count <= 4) {
			return nil
		}
		self.len = Int( Data(slice[..<4]).withUnsafeBytes { $0.load(as: UInt32.self) } )
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

	func readBytesCopy() -> Result<[UInt8], rpckitError> {
		switch self.readBytes() {
		case .success(let val):
			return .success(Array(val[..<val.endIndex]))
		case .failure(let error):
			return .failure(error)
		}
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
					return .failure(rpckitError.varintOverflow)
				}
				return .success(x | UInt64(b) << s)
			}
			x |= UInt64(b & 0x7F) << s
			s += 7
		}
		return .failure(rpckitError.varintOverflow)
	}

	func readVarInt() -> Result<Int64, rpckitError> {
		switch self.readVarUInt() {
		case .success(let val):
			return .success(Int64(val))
		case .failure(let error):
			return .failure(error)
		}
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
		let val = Data(self.buf[self.pos..<self.pos+4]).withUnsafeBytes { $0.load(as: Double.self) }
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

class writableMessage {
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
			let data = Swift.withUnsafeBytes(of: UInt32(self.buf.count)) { Data($0) }
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

protocol rpcMessage {
	func rpcEncode(msg: writableMessage) -> Result<Void, rpckitError>
	func rpcDecode(msg: writableMessage) -> Result<Void, rpckitError>
	func rpcID() -> UInt64
}

protocol rpcCallable {
	func call(server: rpcCallServer) -> rpcMessage
}

protocol rpcCallServer {
	func handle(methodID: UInt64, msg: readableMessage) -> Result<rpcCallable, rpckitError>;
}

class callSlot {
	let callback: (readableMessage) -> Result<Void, rpckitError>

	init(callback: @escaping (readableMessage) -> Result<Void, rpckitError>) {
		self.callback = callback
	}
}

protocol messageWriter {
	func write(wmsg: writableMessage) -> Result<Void, rpckitError>
}

class RPCConnection {
	var nextID: UInt64 = 0
	var slots: [UInt64: callSlot] = [:]
	var servers: [UInt64 : rpcCallServer]
	var writer: messageWriter

	init(writer: messageWriter, servers: [UInt64 : rpcCallServer]) {
		self.servers = servers
		self.writer = writer
	}

	func acquireCallSlot(callback: @escaping (readableMessage) -> Result<Void, rpckitError>) -> UInt64 {
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

	func findAndReleaseCallSlot(id: UInt64) -> callSlot? {
		return self.slots.removeValue(forKey: id)
	}

	func gotMessage(msg: readableMessage) -> Result<Void, rpckitError> {
		do {
			guard let mt = messageType(rawValue: try msg.readVarUInt().get()) else {
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

				return self.writer.write(wmsg: wmsg)
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
			// TODO: Do something!
			return .failure(rpckitError.protocolError)
		}
		return .failure(rpckitError.eof)
	}
}







enum protocolPingpongMethod: UInt64 {
	case SimpleTest = 1
	
}
























// The RPCPingpongClient type is a RPC client for the pingpong protocol.
class RPCPingpongClient {
	let conn: RPCConnection

	init(conn: RPCConnection) {
		self.conn = conn
	}

	func SimpleTest(username: String, password: String, callback: @escaping (Result<(Bool, Bool), Error>) -> ()) -> Result<Void, Error> {

		let _callID = self.conn.acquireCallSlot(callback: { (_rmsg) -> Result<Void, rpckitError> in
		    var success: Bool = false
		    
		    var sortasuccess: Bool = false
		    
			do {
				while true {
					let tag = try _rmsg.readVarUInt().get()
					switch tag {
		        	case (1 << 3 | wireType.varInt.rawValue):
	            		success = try _rmsg.readBool().get()

		        		break
		        	case (2 << 3 | wireType.varInt.rawValue):
	            		sortasuccess = try _rmsg.readBool().get()

		        		break
		        	default:
		        		break
					}
				}
			} catch  {
			}

			callback(.success((success, sortasuccess)))
			return .success(())
		})
		let _wmsg = writableMessage()
		_wmsg.writeVarUInt(value: messageType.methodCall.rawValue)
		_wmsg.writeVarUInt(value: _callID)
		_wmsg.writeVarUInt(value: 1)
		_wmsg.writeVarUInt(value: protocolPingpongMethod.SimpleTest.rawValue)
	    _wmsg.writePBString(fieldNumber: 1, value: username)
	    _wmsg.writePBString(fieldNumber: 2, value: password)

		_wmsg.finish()

		if case .failure(let error) = self.conn.writer.write(wmsg: _wmsg) {
			let _ = self.conn.findAndReleaseCallSlot(id: _callID)
			return .failure(error)
		}

		return .success(())
	}

	
}



