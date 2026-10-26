# References

This directory contains the public Python prototype and skill specifications used to establish parity for the Go runtime.

They are reference material, not shipped runtime code. The committed test vault and captured envelopes are synthetic and safe to publish.

## Structure

- `scripts/` — prototype workflows and deterministic capture helpers;
- `skills/` — workflow descriptions and the public skill manifest; and
- `README.md` — this index.

When behavior is ambiguous, run the relevant prototype against the synthetic fixture and compare its output with the Go parity test. Do not copy material from a live vault into this directory.
