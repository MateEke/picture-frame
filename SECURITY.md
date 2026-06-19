# Security policy

## Supported versions

The frame ships as a single binary with a built-in self-updater that auto-applies any newer
release sharing the current major. Security fixes land as a new patch on the latest major, so the
supported version is always the latest release. Older releases get no backports; update to get the
fix.

## Reporting a vulnerability

Please report security issues privately, not in a public issue or pull request.

- Preferred: open a private advisory under the repository's **Security** tab, **Report a
  vulnerability**. The report and discussion stay private until a fix ships.
- Otherwise, email <ekemate93@gmail.com>.

Include the version, the platform (`linux_armv6`, `armv7`, or `arm64`), and the steps to
reproduce. A proof of concept helps. This is a solo-maintained project, so expect a first reply
within about a week. Please allow time for a fix before disclosing publicly.

## Scope and threat model

The frame is a local-network appliance, not an internet-facing service, and its defaults reflect
that. A few behaviors are intentional, documented design rather than vulnerabilities:

- **Plain HTTP.** The admin interface serves over HTTP with no certificate. Anyone who can watch
  traffic on the network can capture the password or session cookie. Put it behind a TLS reverse
  proxy and segment the Wi-Fi for real protection.
- **Open recovery hotspot.** When no known Wi-Fi is in range, the frame raises an access point so
  you can reconfigure it. That hotspot is open by default. Set an access-point password to close
  it.
- **Stateless sessions.** Signing in issues a signed cookie with a fixed lifetime. Signing out
  clears your cookie but cannot revoke a token already copied elsewhere. Changing the password
  invalidates other sessions.
- **First-come admin password.** Until an admin password is set, anyone who can reach the
  interface can set it. Set one promptly; the installer prompts for it.

These are covered in the [Security guide](https://pictureframe.ekemate.hu/manual/security/).
Reports that only restate them are out of scope. In scope: bypassing a configured password, the
update signature check, or the Wi-Fi credential handling.
