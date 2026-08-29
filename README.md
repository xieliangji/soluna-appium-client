[English](README.md) | [简体中文](README.zh-CN.md)

# soluna-appium-client

A Go Appium client for the Soluna ecosystem.

## Status

This project is under active development and is not yet ready for production use.

The initial implementation will be extracted from the Appium/WebDriver adapter currently used by Soluna.

## Overview

`soluna-appium-client` is a mobile-first Appium client written in Go.

It is designed to provide a reusable and reliable client layer for communicating with Appium through the W3C WebDriver protocol and Appium-specific extensions.

The project focuses on:

- predictable session and command semantics;
- explicit timeout and cancellation handling;
- structured WebDriver error classification;
- bounded request and response processing;
- W3C Actions for mobile gestures;
- reusable Appium capabilities across Soluna projects;
- compatibility testing against real Appium environments.

## Planned Scope

The first development phase will focus on the capabilities already required by Soluna:

- Appium server readiness checks;
- session creation and termination;
- session health probing;
- element lookup;
- element text, attributes, and rectangle retrieval;
- element clearing and text input;
- screenshots and page source retrieval;
- W3C tap, long-press, and swipe actions;
- application activation, termination, and state queries;
- screen recording;
- synchronous script and `mobile:` command execution;
- structured transport and WebDriver errors.

Additional XCUITest and UiAutomator2 capabilities may be introduced as separate extensions.

## Design Principles

- The client library provides protocol mechanisms, not business workflow policies.
- Logical session recovery remains the responsibility of the caller.
- Commands with side effects are not automatically retried.
- Context cancellation and deadlines are propagated through all remote commands.
- Platform-specific features should not unnecessarily expand the common API.
- The core package should remain independent of AI providers and test frameworks.

## Non-Goals

This project does not aim to provide:

- a complete test framework;
- test case orchestration;
- assertions or reporting;
- Appium server installation or process management;
- device discovery and lifecycle management;
- automatic application state recovery;
- automatic retries for failed interaction commands.

These responsibilities belong to Soluna or other higher-level consumers.

## Installation

The public API is not stable yet. Installation instructions will be added with the first usable release.

## Development

Run the standard Go checks with:

```bash
go fmt ./...
go test ./...
go vet ./...