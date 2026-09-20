(() => {
  const q = document.getElementById("q");
  document.addEventListener("keydown", (e) => {
    const tag = (e.target && e.target.tagName) || "";
    const typing = tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      if (q) { q.focus(); q.select(); }
    }
    if (!typing && e.key === "/" && q) {
      e.preventDefault();
      q.focus();
    }
  });

  const origin = window.location.origin;
  document.querySelectorAll("[data-copy-origin]").forEach((btn) => {
    const path = btn.getAttribute("data-copy-origin") || "/mcp";
    btn.addEventListener("click", () => copyText(btn, origin + path));
  });
  document.querySelectorAll("[data-copy]").forEach((btn) => {
    const sel = btn.getAttribute("data-copy");
    const el = sel ? document.querySelector(sel) : null;
    btn.addEventListener("click", () => {
      const text = (el ? el.innerText : "").replaceAll("ORIGIN", origin);
      copyText(btn, text);
    });
  });
  document.querySelectorAll("#mcp-json code, #mcp-json").forEach((el) => {
    if (el.dataset.filled) return;
    el.textContent = el.textContent.replaceAll("ORIGIN", origin);
    el.dataset.filled = "1";
  });

  const ping = document.getElementById("health-run");
  const out = document.getElementById("health-out");
  if (ping && out) {
    const run = () => {
      out.textContent = "…";
      fetch("/healthz")
        .then((r) => r.text().then((t) => ({ ok: r.ok, t })))
        .then(({ ok, t }) => {
          out.textContent = t;
          out.classList.toggle("is-ok", ok);
        })
        .catch((e) => { out.textContent = String(e); });
    };
    ping.addEventListener("click", run);
  }

  const boxes = Array.from(document.querySelectorAll(".pick-box"));
  const go = document.getElementById("cmp-go");
  const a = document.getElementById("cmp-a");
  const b = document.getElementById("cmp-b");
  const sync = () => {
    const on = boxes.filter((x) => x.checked).slice(0, 2);
    boxes.forEach((x) => { x.disabled = !x.checked && on.length >= 2; });
    if (a) a.value = on[0] ? on[0].value : "";
    if (b) b.value = on[1] ? on[1].value : "";
    if (go) go.disabled = on.length !== 2;
  };
  boxes.forEach((x) => x.addEventListener("change", sync));
  sync();

  function copyText(btn, text) {
    const done = () => {
      const prev = btn.textContent;
      btn.textContent = "Copied";
      btn.classList.add("is-copied");
      window.setTimeout(() => {
        btn.textContent = prev;
        btn.classList.remove("is-copied");
      }, 1200);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done).catch(() => fallbackCopy(text, done));
    } else {
      fallbackCopy(text, done);
    }
  }
  function fallbackCopy(text, done) {
    const ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand("copy"); } catch (_) {}
    ta.remove();
    done();
  }
})();
