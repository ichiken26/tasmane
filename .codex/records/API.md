# API

The Go process listens only on loopback. Every OpenAPI operation requires both `InternalBindingAuth` and `WorkerUserContext`. Task transition requires `expectedVersion`; client-supplied user identity is rejected. MCP exposes no delete action.
