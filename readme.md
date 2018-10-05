RpcKit defines a simple RPC system:

- Protocol is defined in Code (Go in this case).
- Client and Server code is generated, and the generated code has zero dependencies outside the regular standard library.
- Documentation can be auto generated based on metadata
- Multiple protocols supported
	- Protobuf messages over SSL connection
	- JSON over HTTP
- Ability to stay connected even in face of disconnects.
- Timeout for rpc calls. (20 seconds by default)


TODO:
	[x] Send preable
	[x] Protobuf encoding/decoding of messages code generation
	[x] Generating code
	[x] Errors handling (both in return values, and in unexpected cases)

	[ ] ability to stay connected even in face of disconnects.
	[ ] Meta code generation (for inspecting properties and summary and such) (if not, just make the schema avaliable)
	[ ] http interface with support for json read/write
	[ ] swift client generation
	[ ] golang http client generation
