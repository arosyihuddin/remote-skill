/* --- shell prompt info (shared with files tab) --- */
var user = "",
  pwd = "",
  dirDisp = "~";
function fetchPromptInfo() {
  return api("POST", "/exec", {
    cmd: ["sh", "-c", 'whoami 2>/dev/null;echo "---";pwd 2>/dev/null'],
    timeout_sec: 5,
  })
    .then(function (r) {
      if (!r || !r.stdout) return;
      var parts = r.stdout.split("---\n");
      user = (parts[0] || "").trim() || "user";
      var home = "/home/" + user;
      pwd = home;
      dirDisp = "~";
      if (parts[1]) {
        pwd = parts[1].trim();
        dirDisp = pwd.replace(new RegExp("^" + home), "~");
      }
    })
    .catch(function () {});
}

/* --- shell xterm.js terminal --- */
var term = null,
  liveWS = null,
  termFit = null;
var shellTermStarted = false,
  shellLiveActive = false;

function startShell() {
  if (!deviceId || shellTermStarted) return;
  shellTermStarted = true;
  term = new Terminal({
    cursorBlink: true,
    cursorStyle: "block",
    fontSize: 14,
    allowTransparency: true,
    theme: {
      background: "#1e1e1e",
      foreground: "#d4d4d4",
      cursor: "#d4d4d4",
      selectionBackground: "#264f78",
    },
  });
  var fa = new FitAddon.FitAddon();
  term.loadAddon(fa);
  termFit = fa;
  var c = $("terminal-container");
  c.innerHTML = "";
  term.open(c);
  fa.fit();
  setTimeout(function () {
    fa.fit();
    term.focus();
  }, 50);
  term.onData(function (d) {
    if (liveWS && liveWS.readyState === WebSocket.OPEN && shellLiveActive) {
      var b = new Uint8Array(1 + d.length);
      b[0] = 0x00;
      for (var i = 0; i < d.length; i++) b[i + 1] = d.charCodeAt(i);
      liveWS.send(b);
    }
  });
  term.onResize(function (s) {
    if (liveWS && liveWS.readyState === WebSocket.OPEN && shellLiveActive) {
      var b = new Uint8Array(5);
      b[0] = 0x02;
      b[1] = (s.cols >> 8) & 0xff;
      b[2] = s.cols & 0xff;
      b[3] = (s.rows >> 8) & 0xff;
      b[4] = s.rows & 0xff;
      liveWS.send(b);
    }
  });
  connectWS();
}

function connectWS() {
  shellLiveActive = false;
  if (liveWS) {
    liveWS.onclose = null;
    liveWS.close();
    liveWS = null;
  }
  if (!deviceId) return;
  var p = location.protocol === "https:" ? "wss:" : "ws:";
  liveWS = new WebSocket(p + "//" + location.host + "/live");
  liveWS.binaryType = "arraybuffer";
  liveWS.onopen = function () {
    liveWS.send(
      JSON.stringify({
        type: "hello",
        id: "hello-1",
        payload: { device_id: deviceId },
      }),
    );
  };
  liveWS.onmessage = function (e) {
    if (typeof e.data === "string") {
      try {
        var f = JSON.parse(e.data);
        if (f.type === "ack") {
          shellLiveActive = true;
          liveWS.send(
            JSON.stringify({
              type: "live",
              id: "live-req",
              payload: { cols: term.cols, rows: term.rows },
            }),
          );
        } else if (f.type === "error") {
          var ep = JSON.parse(f.payload);
          term.write(
            "\r\n\x1b[31m[error: " + (ep.message || "unknown") + "]\x1b[0m\r\n",
          );
        }
      } catch (_) {}
    } else if (e.data instanceof ArrayBuffer && shellLiveActive) {
      var v = new Uint8Array(e.data);
      if (v[0] === 0x01) {
        term.write(new Uint8Array(e.data, 1));
      } else if (v[0] === 0x03) {
        var m =
          e.data.byteLength > 1
            ? new TextDecoder().decode(e.data.slice(1))
            : "";
        term.write("\r\n[session ended" + (m ? ": " + m : "") + "]\r\n");
        shellLiveActive = false;
      }
    }
  };
  liveWS.onclose = function () {
    if (shellLiveActive) term.write("\r\n[connection closed]\r\n");
    shellLiveActive = false;
    liveWS = null;
  };
  liveWS.onerror = function () {
    if (term) term.write("\r\n\x1b[31m[WebSocket error]\x1b[0m\r\n");
  };
}

function stopShell() {
  if (liveWS) {
    liveWS.onclose = null;
    liveWS.close();
    liveWS = null;
  }
  shellLiveActive = false;
  if (term) {
    term.dispose();
    term = null;
  }
  termFit = null;
  shellTermStarted = false;
}

function onTabShell() {
  if (!deviceId) return;
  if (!shellTermStarted) startShell();
  else if (termFit) {
    termFit.fit();
    term.focus();
  }
}

function onShellDeviceChange() {
  if (!shellTermStarted) return;
  shellLiveActive = false;
  if (liveWS) {
    liveWS.onclose = null;
    liveWS.close();
    liveWS = null;
  }
  if (
    document.querySelector('[data-tab="shell"]') &&
    document.querySelector('[data-tab="shell"]').classList.contains("active")
  )
    connectWS();
}
