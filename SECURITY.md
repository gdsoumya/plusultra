# Security Policy

PlusUltra is self-hosted: the server holds your training, nutrition and check-in data, including
photos, and talks to no other service except the public catalog datasets at build time and
Open Food Facts for barcode lookups.

## Reporting a vulnerability

Please do not open a public issue for a security problem. Use GitHub's private vulnerability
reporting on this repository (Security, then Report a vulnerability). You will get a response
within a week.

## Scope notes

- The server listens on plain HTTP by design; put a reverse proxy with TLS in front of it for
  anything beyond a trusted network.
- Sessions and API keys are random tokens stored hashed; there is no session secret to rotate.
- Registration is invite-only by default. `REGISTRATION=open` lets anyone with network access
  create an account.
