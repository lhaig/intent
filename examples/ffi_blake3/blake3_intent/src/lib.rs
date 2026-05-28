//! Tiny wrapper around the `blake3` crate exposing a single
//! Intent-bridgeable signature: `String -> String`.
//!
//! Intent's FFI bridge only permits the types listed in ADR 0028.
//! `blake3::hash` returns a `blake3::Hash`, which is not bridgeable,
//! so we hex-encode it here and return an owned `String`.

pub fn hash_hex(input: String) -> String {
    blake3::hash(input.as_bytes()).to_hex().to_string()
}
