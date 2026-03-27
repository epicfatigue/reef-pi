# reef-pi

<p align="center">
  <b>Advanced Reef Aquarium Controller for Raspberry Pi</b><br>
  Instrumentation-Focused Fork with Enhanced Driver & Platform Support
</p>

<p align="center">
  <a href="https://paypal.me/miwoodrow">
    <img src="https://img.shields.io/badge/Support-PayPal-blue?logo=paypal" />
  </a>
</p>

<p align="center">
  <img src="https://img.shields.io/github/release/reef-pi/reef-pi.svg" />
  <img src="https://github.com/reef-pi/reef-pi/workflows/go/badge.svg?branch=main" />
  <img src="https://github.com/reef-pi/reef-pi/workflows/jest/badge.svg?branch=main" />
  <img src="https://github.com/reef-pi/reef-pi/workflows/smoke/badge.svg?branch=main" />
  <img src="https://github.com/reef-pi/reef-pi/workflows/deb/badge.svg?branch=main" />
</p>

<p align="center">
  <img src="https://codecov.io/gh/reef-pi/reef-pi/branch/main/graph/badge.svg" />
  <img src="https://goreportcard.com/badge/reef-pi/reef-pi" />
  <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" />
  <img src="https://godoc.org/github.com/reef-pi/reef-pi?status.svg" />
</p>

---

> ⚠️ **This is a personal fork of the original reef-pi project.**  
> For official releases and stable builds, please visit:  
> 👉 https://github.com/reef-pi/reef-pi

---

## Overview

[reef-pi](http://reef-pi.com) is an open-source reef aquarium controller built for the Raspberry Pi.

This fork extends the original architecture with enhanced instrumentation, expanded driver capability, and refined platform support — focused on precision monitoring, salinity modeling, and hardware abstraction improvements.

It is designed for reef keepers who want deeper visibility, tighter calibration control, and expanded Raspberry Pi compatibility.

---

## Key Enhancements in This Fork

This repository builds upon the original reef-pi foundation and introduces:

- Advanced conductivity and salinity driver support  
- Improved temperature compensation modeling  
- Hardware abstraction refinements  
- Expanded Raspberry Pi platform compatibility  
- Experimental calibration and instrumentation tooling  
- Enhanced diagnostics and validation workflows  

All credit for the original architecture, design, and ecosystem belongs to the reef-pi maintainers and contributors.

---

## Installation

### Quick Install (Raspberry Pi OS – Debian Trixie)

On a fresh Raspberry Pi OS system, run:

```bash
curl -fsSL https://raw.githubusercontent.com/epicfatigue/reef-pi/main/install.sh | sudo bash
```
---

### What the Installer Does

The automated installer performs a complete setup:

- Updates the operating system  
- Installs all required dependencies (Go, Node.js, yarn, git, build tools)  
- Creates a dedicated **`reefpi` system user**  
  - ⚠️ Reserved for the service — do **not** create or modify manually  
- Creates required directories under `/opt` and `/var/lib`  
- Clones all required repositories (`reef-pi`, `drivers`, `hal`, `rpi`)  
- Wires local Go module replacements for sibling repositories  
- Builds the frontend (if enabled)  
- Compiles and installs the backend binary  
- Creates and enables a `systemd` service  
- Starts reef-pi automatically on boot  

The result is a fully compiled, service-managed installation running under a restricted system account.

After installation, access reef-pi at:

```
http://<your-pi-ip>:8080
```

---

## Supporting Development

If this fork helps your reef system and you would like to support continued hardware testing, driver development, and long-term validation:

👉 **https://paypal.me/miwoodrow**

Donations contribute toward:

- Hardware acquisition for driver testing  
- Probe validation and calibration research  
- Multi-platform Raspberry Pi compatibility  
- Long-term stability and drift analysis  

Support is optional. This project will always remain open source.

Please also consider supporting the original reef-pi maintainers who made this ecosystem possible.
