# Cloudflare ingress implementation note

Requirement:
- Browser -> Worker is public application ingress.
- Worker -> Go Container is internal-only.
- Go Container must not expose an independently reachable public application endpoint.
- Worker/Container Binding is the application connection boundary.
- Cloudflare Tunnel/internal routing may be used where required by deployment topology.

Security tests must validate the effective property (no direct public Go ingress and no trust in caller-supplied user IDs), rather than assuming a header alone proves origin.
