package transport

// ALPN identifies the Ashrix QUIC application protocol. Both endpoints must
// negotiate the same value before exchanging Ashrix control/data streams.
const ALPN = "ashrix/1"
