# Project Stargaze

Project Stargaze aims to serve as a proof of concept for the protocol design of Hysteria 2. Some key design objectives
are:

- The protocol must appear identical to HTTP/3, both to middleboxes and active probers.
- The server should have a set of special "hidden endpoints" that require an authentication token in the header to
  access:
    - `/hysteria-xxx/knock` for "knocking" (to enable proxy capability; this must be called first, before any other
      endpoint below)
    - `/hysteria-xxx/cc` for updating CC info (such as up/down speed and the CC algorithm to use)
    - maybe more...
- Once the server's proxy capability is enabled, the client can send proxy requests using a special stream type.
- The server should still serve HTTP for other streams, especially for the "hidden endpoints" above, in case the client
  needs to update CC info, etc.
- If a client doesn't knock or the knock fails, the server must reject special proxy streams and behave as a standard
  HTTP/3 server, serving some legitimate-looking content.
- The server should be able to serve legitimate-looking content for the "hidden endpoints" above, in case the client
  doesn't knock or the knock fails.
    - String mode
    - Static file mode
    - Reverse proxy mode