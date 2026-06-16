#!/usr/bin/env node

// Exercise the request-targeted broadcast reuse scenario through the public
// network surface, not package internals. The script builds a temporary JaWS
// server, creates pages with HTTP GET, connects with raw WebSocket handshakes,
// sends browser-shaped Click frames, and watches whether a Request.Alert queued
// by an old WebSocket is delivered to a later WebSocket that reused the same
// pooled *Request.
//
// Usage:
//   node scripts/exercise-request-dest-reuse.mjs
//   ATTEMPTS=2000 PER_ATTEMPT_MS=10 ALERTS=256 node scripts/exercise-request-dest-reuse.mjs
//
// Exit status is zero when the stress run does not observe the network-level
// race. If it reproduces, the script prints the old and new jawsKey values and
// exits non-zero.

import { spawn, spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { Socket } from "node:net";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const attempts = Number.parseInt(process.env.ATTEMPTS || "500", 10);
const perAttemptMs = Number.parseInt(process.env.PER_ATTEMPT_MS || "40", 10);
const alerts = Math.max(1, Number.parseInt(process.env.ALERTS || "64", 10));

const serverSource = String.raw`package main

import (
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/linkdata/jaws"
)

var triggerCount atomic.Uint64

const alertBurst uint64 = __ALERT_BURST__

type triggerUI struct{}

func (triggerUI) JawsRender(elem *jaws.Element, w io.Writer, _ []any) error {
	_, err := fmt.Fprintf(w, "<button id=\"%s\">trigger</button>", html.EscapeString(elem.Jid().String()))
	return err
}

func (triggerUI) JawsUpdate(*jaws.Element) {}

func (triggerUI) JawsClick(elem *jaws.Element, _ jaws.Click) error {
	triggerCount.Add(1)
	for i := uint64(0); i < alertBurst; i++ {
		elem.Alert("warning", fmt.Sprintf("stale request-targeted message from %s #%d", elem.JawsKeyString(), i))
	}
	elem.Cancel(errors.New("intentional close after request-targeted broadcast"))
	return nil
}

func main() {
	jw, err := jaws.New()
	if err != nil {
		log.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	mux := http.NewServeMux()
	mux.Handle("/jaws/", jw)
	mux.HandleFunc("/hits", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, triggerCount.Load())
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rq := jw.NewRequest(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><html><head>")
		if err := rq.HeadHTML(w); err != nil {
			log.Print(err)
			return
		}
		_, _ = io.WriteString(w, "</head><body>")
		elem := rq.NewElement(triggerUI{})
		if err := elem.JawsRender(w, nil); err != nil {
			log.Print(err)
			return
		}
		if err := rq.TailHTML(w); err != nil {
			log.Print(err)
			return
		}
		_, _ = io.WriteString(w, "</body></html>")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("LISTEN http://" + ln.Addr().String())
	log.Fatal(http.Serve(ln, mux))
}
`.replace("__ALERT_BURST__", String(alerts));

class RawWebSocket {
	constructor(socket, buffer) {
		this.socket = socket;
		this.buffer = buffer || Buffer.alloc(0);
		this.waiters = [];
		this.closed = false;
		socket.on("data", (chunk) => {
			this.buffer = Buffer.concat([this.buffer, chunk]);
			this.#wake();
		});
		socket.on("close", () => {
			this.closed = true;
			this.#wake();
		});
		socket.on("error", () => {
			this.closed = true;
			this.#wake();
		});
	}

	#wake() {
		for (const wake of this.waiters.splice(0)) {
			wake();
		}
	}

	async #waitForData(timeoutMs) {
		if (this.buffer.length > 0 || this.closed) {
			return;
		}
		await new Promise((resolve) => {
			const timer = setTimeout(resolve, timeoutMs);
			this.waiters.push(() => {
				clearTimeout(timer);
				resolve();
			});
		});
	}

	sendText(text) {
		this.socket.write(clientTextFrame(Buffer.from(text)));
	}

	destroy() {
		this.socket.destroy();
	}

	end() {
		this.socket.end();
	}

	async readText(timeoutMs) {
		const deadline = Date.now() + timeoutMs;
		for (;;) {
			const frame = tryReadFrame(this);
			if (frame) {
				if (frame.opcode === 0x8) {
					this.closed = true;
					return null;
				}
				if (frame.opcode === 0x1) {
					return frame.payload.toString("utf8");
				}
				continue;
			}
			const left = deadline - Date.now();
			if (left <= 0 || this.closed) {
				return null;
			}
			await this.#waitForData(left);
		}
	}
}

function clientTextFrame(payload) {
	const mask = randomBytes(4);
	let header;
	if (payload.length < 126) {
		header = Buffer.from([0x81, 0x80 | payload.length]);
	} else if (payload.length <= 0xffff) {
		header = Buffer.alloc(4);
		header[0] = 0x81;
		header[1] = 0x80 | 126;
		header.writeUInt16BE(payload.length, 2);
	} else {
		throw new Error("payload too large");
	}
	const masked = Buffer.alloc(payload.length);
	for (let i = 0; i < payload.length; i++) {
		masked[i] = payload[i] ^ mask[i % 4];
	}
	return Buffer.concat([header, mask, masked]);
}

function tryReadFrame(ws) {
	const b = ws.buffer;
	if (b.length < 2) {
		return null;
	}
	const opcode = b[0] & 0x0f;
	const masked = (b[1] & 0x80) !== 0;
	let length = b[1] & 0x7f;
	let offset = 2;
	if (length === 126) {
		if (b.length < offset + 2) {
			return null;
		}
		length = b.readUInt16BE(offset);
		offset += 2;
	} else if (length === 127) {
		if (b.length < offset + 8) {
			return null;
		}
		const big = b.readBigUInt64BE(offset);
		if (big > BigInt(Number.MAX_SAFE_INTEGER)) {
			throw new Error("frame too large");
		}
		length = Number(big);
		offset += 8;
	}
	let mask;
	if (masked) {
		if (b.length < offset + 4) {
			return null;
		}
		mask = b.subarray(offset, offset + 4);
		offset += 4;
	}
	if (b.length < offset + length) {
		return null;
	}
	const payload = Buffer.from(b.subarray(offset, offset + length));
	ws.buffer = b.subarray(offset + length);
	if (masked) {
		for (let i = 0; i < payload.length; i++) {
			payload[i] ^= mask[i % 4];
		}
	}
	return { opcode, payload };
}

async function openRawWebSocket(baseURL, jawsKey) {
	const u = new URL(baseURL);
	const socket = new Socket();
	await new Promise((resolve, reject) => {
		socket.once("connect", resolve);
		socket.once("error", reject);
		socket.connect(Number(u.port), u.hostname);
	});
	const wsKey = randomBytes(16).toString("base64");
	const path = `/jaws/${encodeURIComponent(jawsKey)}`;
	socket.write([
		`GET ${path} HTTP/1.1`,
		`Host: ${u.host}`,
		"Upgrade: websocket",
		"Connection: Upgrade",
		`Sec-WebSocket-Key: ${wsKey}`,
		"Sec-WebSocket-Version: 13",
		`Origin: ${u.origin}`,
		"",
		"",
	].join("\r\n"));

	let buffer = Buffer.alloc(0);
	while (!buffer.includes("\r\n\r\n")) {
		const chunk = await new Promise((resolve, reject) => {
			socket.once("data", resolve);
			socket.once("error", reject);
			socket.once("close", () => reject(new Error("socket closed during handshake")));
		});
		buffer = Buffer.concat([buffer, chunk]);
	}
	const headerEnd = buffer.indexOf("\r\n\r\n");
	const header = buffer.subarray(0, headerEnd).toString("latin1");
	if (!header.startsWith("HTTP/1.1 101 ")) {
		throw new Error(`websocket handshake failed:\n${header}`);
	}
	return new RawWebSocket(socket, buffer.subarray(headerEnd + 4));
}

async function fetchPage(baseURL) {
	const res = await fetch(baseURL + "/");
	const body = await res.text();
	const match = body.match(/<meta name="jawsKey" content="([^"]+)">/);
	if (!match) {
		throw new Error("page did not contain jawsKey meta tag");
	}
	return match[1];
}

function waitForServer(proc) {
	return new Promise((resolve, reject) => {
		let stdout = "";
		let stderr = "";
		const timer = setTimeout(() => reject(new Error(`server did not start\n${stderr}`)), 15000);
		proc.stdout.on("data", (chunk) => {
			stdout += chunk.toString();
			const match = stdout.match(/LISTEN (http:\/\/127\.0\.0\.1:\d+)/);
			if (match) {
				clearTimeout(timer);
				resolve(match[1]);
			}
		});
		proc.stderr.on("data", (chunk) => {
			stderr += chunk.toString();
		});
		proc.on("exit", (code) => {
			clearTimeout(timer);
			reject(new Error(`server exited with ${code}\n${stderr}`));
		});
	});
}

async function attempt(baseURL, n) {
	const key1 = await fetchPage(baseURL);
	const ws1 = await openRawWebSocket(baseURL, key1);
	const t0 = process.hrtime.bigint();
	ws1.sendText("Click\tJid.1\t1 2 0 trigger\n");
	ws1.end();

	const key2 = await fetchPage(baseURL);
	const ws2 = await openRawWebSocket(baseURL, key2);
	const connectedNs = process.hrtime.bigint() - t0;

	let reproduced = false;
	const deadline = Date.now() + perAttemptMs;
	while (Date.now() < deadline) {
		const frame = await ws2.readText(Math.max(1, deadline - Date.now()));
		if (!frame) {
			break;
		}
		if (frame.includes("stale request-targeted message from " + key1)) {
			console.log(`reproduced on attempt ${n}: old key ${key1} alert arrived on new key ${key2}`);
			console.log(frame.trim());
			reproduced = true;
			break;
		}
	}

	ws1.destroy();
	ws2.destroy();
	return { reproduced, connectedNs };
}

function formatDuration(ns) {
	const value = Number(ns) / 1e6;
	if (value < 1) {
		return `${(Number(ns) / 1e3).toFixed(1)}us`;
	}
	return `${value.toFixed(3)}ms`;
}

function percentile(sorted, pct) {
	if (sorted.length === 0) {
		return 0n;
	}
	const idx = Math.min(sorted.length - 1, Math.floor((sorted.length - 1) * pct));
	return sorted[idx];
}

async function main() {
	const tmp = mkdtempSync(join(tmpdir(), "jaws-http-ws-repro-"));
	let server;
	try {
		writeFileSync(join(tmp, "go.mod"), `module jawsrepro

go 1.25

require github.com/linkdata/jaws v0.0.0

replace github.com/linkdata/jaws => ${repoRoot}
`);
		writeFileSync(join(tmp, "main.go"), serverSource);

		const tidy = spawnSync("go", ["mod", "tidy"], {
			cwd: tmp,
			encoding: "utf8",
		});
		if (tidy.status !== 0) {
			throw new Error(`go mod tidy failed\n${tidy.stdout}${tidy.stderr}`);
		}

		const serverPath = join(tmp, "jaws-http-ws-repro-server");
		const build = spawnSync("go", ["build", "-o", serverPath, "."], {
			cwd: tmp,
			encoding: "utf8",
		});
		if (build.status !== 0) {
			throw new Error(`go build failed\n${build.stdout}${build.stderr}`);
		}

		server = spawn(serverPath, [], {
			cwd: tmp,
			stdio: ["ignore", "pipe", "pipe"],
		});
		const baseURL = await waitForServer(server);
		console.log(`server: ${baseURL}`);
		console.log(`attempts: ${attempts}, alerts per trigger: ${alerts}, per-attempt read window: ${perAttemptMs}ms`);

		const connectTimes = [];
		for (let i = 1; i <= attempts; i++) {
			const result = await attempt(baseURL, i);
			connectTimes.push(result.connectedNs);
			if (result.reproduced) {
				connectTimes.sort((a, b) => Number(a - b));
				console.log(`fastest old-click-to-new-websocket time: ${formatDuration(connectTimes[0])}`);
				process.exitCode = 1;
				return;
			}
			if (i % 50 === 0) {
				console.log(`attempted ${i}...`);
			}
		}
		connectTimes.sort((a, b) => Number(a - b));
		if (connectTimes.length > 0) {
			console.log(`old-click-to-new-websocket timing: min=${formatDuration(connectTimes[0])}, p50=${formatDuration(percentile(connectTimes, 0.50))}, p95=${formatDuration(percentile(connectTimes, 0.95))}`);
		}
		const hits = await fetch(baseURL + "/hits").then((res) => res.text());
		console.log(`triggered events observed by server: ${hits.trim()}`);
		console.log("no HTTP/WebSocket reproduction observed");
	} finally {
		if (server) {
			server.kill("SIGTERM");
		}
		rmSync(tmp, { recursive: true, force: true });
	}
}

main().catch((err) => {
	console.error(err.stack || err.message);
	process.exit(2);
});
