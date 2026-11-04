// fsshim.js — an in-memory file system for GameViber's published editor
// (adapted from goloco's web/fsshim.js; the same author).
//
// Go's js/wasm runtime routes os.Open / ReadDir / Stat / Read / Write through
// the Node-style globalThis.fs object (src/syscall/fs_js.go); browsers have
// none, so this installs a working one in memory. The editor's project lives
// at /w/project; every file under it is kept in localStorage (one key a file,
// base64), so edits survive a reload. Load it before wasm_exec.js.
"use strict";

(() => {
	const S_IFDIR = 0o040000, S_IFREG = 0o100000;
	const O_WRONLY = 1, O_RDWR = 2, O_CREAT = 64, O_EXCL = 128, O_TRUNC = 512, O_APPEND = 1024, O_DIRECTORY = 65536;

	const files = new Map();   // absolute path -> Uint8Array
	const dirs = new Map();    // absolute path -> Set of entry names
	const fds = new Map();     // fd -> {path, pos, dir, append}
	const misses = new Set();
	const lazy = new Map();    // absolute path -> {url, size}: fetched on first open (music)
	let nextFd = 10;
	let cwd = "/w";
	let out = { 1: "", 2: "" };
	const decoder = new TextDecoder("utf-8");

	const norm = (p) => {
		if (!p.startsWith("/")) p = cwd + "/" + p;
		const parts = [];
		for (const s of p.split("/")) {
			if (s === "" || s === ".") continue;
			if (s === "..") parts.pop(); else parts.push(s);
		}
		return "/" + parts.join("/");
	};
	const parent = (p) => p.slice(0, p.lastIndexOf("/")) || "/";
	const base = (p) => p.slice(p.lastIndexOf("/") + 1);

	const mkdirp = (p) => {
		while (!dirs.has(p)) {
			dirs.set(p, new Set());
			if (p === "/") break;
			dirs.get(parent(p)) || mkdirp(parent(p));
			dirs.get(parent(p)).add(base(p));
			p = parent(p);
		}
	};
	mkdirp("/w");

	const addFile = (path, data) => {
		path = norm(path);
		mkdirp(parent(path));
		dirs.get(parent(path)).add(base(path));
		files.set(path, data);
	};

	// GameViber's published editor: an in-memory file system for Go's js/wasm
// (adapted from goloco's web/fsshim.js), the project kept in localStorage.
// The game's config (HOME=/w: os.UserConfigDir is /w/.config) is kept in
	// the browser's localStorage, so a setting chosen in the game (the
	// language …) survives a reload. Each file is one key, base64.
	const KEEP = "/w/project/", KEEP_KEY = "gameviber-fs:";
	const keep = (path) => {
		if (!path.startsWith(KEEP) || !files.has(path)) return;
		const hook = globalThis.GameviberFS && globalThis.GameviberFS.onChange;
		if (hook) hook(path, files.get(path)); // folder mode: written to the folder on disk
		if (globalThis.GameviberFS && globalThis.GameviberFS.folderMode) return; // the folder keeps it, not localStorage
		try {
			let bin = "";
			for (const b of files.get(path)) bin += String.fromCharCode(b);
			localStorage.setItem(KEEP_KEY + path, btoa(bin));
		} catch (e) { /* no storage: the setting lasts until the page closes */ }
	};
	const restoreKept = () => {
		try {
			for (let i = 0; i < localStorage.length; i++) {
				const k = localStorage.key(i);
				if (!k || !k.startsWith(KEEP_KEY)) continue;
				const bin = atob(localStorage.getItem(k));
				const data = new Uint8Array(bin.length);
				for (let j = 0; j < bin.length; j++) data[j] = bin.charCodeAt(j);
				addFile(k.slice(KEEP_KEY.length), data);
			}
		} catch (e) { /* no storage */ }
	};

	const err = (code, path) => {
		if (code === "ENOENT" && path) misses.add(path);
		const e = new Error(code + (path ? ": " + path : ""));
		e.code = code;
		return e;
	};

	// addLazy registers a file the game may open later; it is fetched (gzip)
	// the first time it is opened, so it costs nothing until then.
	const addLazy = (path, url, size) => {
		path = norm(path);
		mkdirp(parent(path));
		dirs.get(parent(path)).add(base(path));
		lazy.set(path, { url, size });
	};

	const fetchLazy = async (path) => {
		const { url } = lazy.get(path);
		const resp = await fetch(url, { cache: "no-cache" }); // revalidate: a rebuilt page must not run stale files
		if (!resp.ok) throw new Error(url + ": HTTP " + resp.status);
		// .gz: a gzipped file; anything else (the Opus music) as it is.
		const s = url.endsWith(".gz") ? resp.body.pipeThrough(new DecompressionStream("gzip")) : resp.body;
		const data = new Uint8Array(await new Response(s).arrayBuffer());
		files.set(path, data);
		lazy.get(path) && (lazy.get(path).size = data.length);
		lazy.delete(path);
		console.log("fetched " + path);
	};

	const statOf = (path) => {
		const isDir = dirs.has(path);
		const f = files.get(path) || (lazy.has(path) ? { length: lazy.get(path).size } : null);
		if (!isDir && !f) return null;
		return {
			dev: 0, ino: 0, mode: isDir ? (S_IFDIR | 0o755) : (S_IFREG | 0o644), nlink: 1,
			uid: 0, gid: 0, rdev: 0, size: isDir ? 0 : f.length, blksize: 4096,
			blocks: isDir ? 0 : Math.ceil(f.length / 512), atimeMs: 0, mtimeMs: 0, ctimeMs: 0,
			isDirectory() { return isDir; },
		};
	};

	const console_out = (fd, s) => {
		out[fd] += s;
		const nl = out[fd].lastIndexOf("\n");
		if (nl !== -1) {
			const lines = out[fd].substring(0, nl);
			out[fd] = out[fd].substring(nl + 1);
			(fd === 2 ? console.warn : console.log)(lines);
			if (globalThis.GameviberFS.onLog) globalThis.GameviberFS.onLog(lines);
		}
	};

	const writeAt = (fd, buf, offset, length, position) => {
		const f = fds.get(fd);
		const old = files.get(f.path) || new Uint8Array(0);
		const at = f.append ? old.length : (position ?? f.pos);
		const end = at + length;
		let data = old;
		if (end > old.length) { data = new Uint8Array(end); data.set(old); }
		data.set(buf.subarray(offset, offset + length), at);
		files.set(f.path, data);
		if (position == null) f.pos = end;
		f.dirty = true;
		return length;
	};

	const fs = {
		constants: { O_WRONLY, O_RDWR, O_CREAT, O_TRUNC, O_APPEND, O_EXCL, O_DIRECTORY },
		writeSync(fd, buf) {
			if (fd === 1 || fd === 2) { console_out(fd, decoder.decode(buf)); return buf.length; }
			return writeAt(fd, buf, 0, buf.length, null);
		},
		write(fd, buf, offset, length, position, cb) {
			if (fd === 1 || fd === 2) { console_out(fd, decoder.decode(buf.subarray(offset, offset + length))); cb(null, length); return; }
			if (!fds.has(fd)) { cb(err("EBADF")); return; }
			cb(null, writeAt(fd, buf, offset, length, position));
		},
		open(path, flags, mode, cb) {
			path = norm(path);
			if (lazy.has(path)) {
				fetchLazy(path).then(() => this.open(path, flags, mode, cb), (e) => { console.warn(e); cb(err("EIO", path)); });
				return;
			}
			const exists = files.has(path) || dirs.has(path);
			if (!exists && !(flags & O_CREAT)) { cb(err("ENOENT", path)); return; }
			if (exists && (flags & O_CREAT) && (flags & O_EXCL)) { cb(err("EEXIST", path)); return; }
			if ((flags & O_DIRECTORY) && !dirs.has(path)) { cb(err(exists ? "ENOTDIR" : "ENOENT", path)); return; }
			if (!exists) {
				if (!dirs.has(parent(path))) { cb(err("ENOENT", path)); return; }
				addFile(path, new Uint8Array(0));
			} else if ((flags & O_TRUNC) && files.has(path)) {
				files.set(path, new Uint8Array(0));
			}
			const fd = nextFd++;
			fds.set(fd, { path, pos: 0, dir: dirs.has(path), append: !!(flags & O_APPEND) });
			cb(null, fd);
		},
		read(fd, buffer, offset, length, position, cb) {
			const f = fds.get(fd);
			if (!f) { cb(err("EBADF")); return; }
			if (f.dir) { cb(err("EISDIR", f.path)); return; }
			const data = files.get(f.path);
			const at = position ?? f.pos;
			const n = Math.max(0, Math.min(length, data.length - at));
			buffer.set(data.subarray(at, at + n), offset);
			if (position == null) f.pos += n;
			cb(null, n);
		},
		close(fd, cb) {
			const f = fds.get(fd);
			if (f && f.dirty) keep(f.path);
			fds.delete(fd); cb(null);
		},
		fstat(fd, cb) {
			const f = fds.get(fd);
			if (!f) { cb(err("EBADF")); return; }
			cb(null, statOf(f.path));
		},
		stat(path, cb) { path = norm(path); const s = statOf(path); s ? cb(null, s) : cb(err("ENOENT", path)); },
		lstat(path, cb) { this.stat(path, cb); },
		readdir(path, cb) {
			path = norm(path);
			const d = dirs.get(path);
			d ? cb(null, [...d].sort()) : cb(err("ENOENT", path));
		},
		mkdir(path, perm, cb) {
			path = norm(path);
			if (dirs.has(path) || files.has(path)) { cb(err("EEXIST", path)); return; }
			if (!dirs.has(parent(path))) { cb(err("ENOENT", path)); return; }
			mkdirp(path); cb(null);
		},
		unlink(path, cb) {
			path = norm(path);
			if (!files.delete(path)) { cb(err("ENOENT", path)); return; }
			dirs.get(parent(path)).delete(base(path));
			try { localStorage.removeItem(KEEP_KEY + path); } catch (e) { /* no storage */ }
			if (globalThis.GameviberFS && globalThis.GameviberFS.onDelete) globalThis.GameviberFS.onDelete(path);
			cb(null);
		},
		rename(from, to, cb) {
			from = norm(from); to = norm(to);
			const data = files.get(from);
			if (!data) { cb(err("ENOENT", from)); return; }
			files.delete(from); dirs.get(parent(from)).delete(base(from));
			addFile(to, data); keep(to); cb(null);
		},
		rmdir(path, cb) {
			path = norm(path);
			const d = dirs.get(path);
			if (!d) { cb(err("ENOENT", path)); return; }
			if (d.size) { cb(err("ENOTEMPTY", path)); return; }
			dirs.delete(path); dirs.get(parent(path))?.delete(base(path)); cb(null);
		},
		fsync(fd, cb) { cb(null); },
		chmod(p, m, cb) { cb(null); }, fchmod(fd, m, cb) { cb(null); },
		chown(p, u, g, cb) { cb(null); }, fchown(fd, u, g, cb) { cb(null); }, lchown(p, u, g, cb) { cb(null); },
		utimes(p, a, m, cb) { cb(null); },
		ftruncate(fd, len, cb) { const f = fds.get(fd); files.set(f.path, (files.get(f.path) || new Uint8Array(0)).slice(0, len)); cb(null); },
		truncate(p, len, cb) { p = norm(p); files.set(p, (files.get(p) || new Uint8Array(0)).slice(0, len)); cb(null); },
		readlink(p, cb) { cb(err("EINVAL")); }, link(p, l, cb) { cb(err("ENOSYS")); }, symlink(p, l, cb) { cb(err("ENOSYS")); },
	};

	globalThis.fs = fs;
	globalThis.process = {
		getuid() { return -1; }, getgid() { return -1; }, geteuid() { return -1; }, getegid() { return -1; },
		getgroups() { return []; }, pid: 1, ppid: 0, umask() { return 0o22; },
		cwd() { return cwd; },
		chdir(p) { p = norm(p); if (!dirs.has(p)) throw err("ENOENT", p); cwd = p; },
	};
	globalThis.path = { resolve(...segs) { return norm(segs.join("/")); } };
	// clearProject empties /w/project in memory (folder mode loads a folder there instead).
	const clearProject = () => {
		for (const p of [...files.keys()]) if (p.startsWith(KEEP)) files.delete(p);
		for (const p of [...dirs.keys()]) if (p.startsWith(KEEP)) dirs.delete(p);
		mkdirp("/w/project");
	};
	globalThis.GameviberFS = { addFile, files, dirs, clearProject, onLog: null, onChange: null, onDelete: null, folderMode: false };
	restoreKept();
})();
